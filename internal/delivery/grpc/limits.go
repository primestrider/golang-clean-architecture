package grpc

import (
	"context"
	"net"
	"net/netip"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// timeoutInterceptor memberi batas waktu pada setiap RPC, setara dengan
// timeout request di API HTTP. Query database yang memakai ctx ikut berhenti.
func timeoutInterceptor(d time.Duration) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if d <= 0 {
			return handler(ctx, req)
		}
		ctx, cancel := context.WithTimeout(ctx, d)
		defer cancel()
		return handler(ctx, req)
	}
}

// Batas memori limiter: jumlah maksimum key yang dilacak dan seberapa sering
// key yang tidak aktif dibersihkan.
const (
	defaultMaxLimiterKeys = 10000
	limiterSweepInterval  = time.Minute
	limiterIdleTTL        = 10 * time.Minute
	overflowKey           = "overflow"
)

// ipLimiter adalah rate limiter token bucket per client, disimpan di memori
// proses ini (untuk beberapa instance, batasnya berlaku per instance).
//
// Jumlah key dibatasi keras (maxKeys). Bila penuh, client baru berbagi satu
// bucket "overflow", sehingga banjir alamat palsu tidak bisa menghabiskan
// memori, dan dampaknya hanya mengenai client yang berada di bucket itu.
type ipLimiter struct {
	mu        sync.Mutex
	perKey    map[string]*limiterEntry
	overflow  *rate.Limiter
	limit     rate.Limit
	burst     int
	maxKeys   int
	lastSweep time.Time
}

// limiterEntry adalah bucket satu client beserta waktu terakhir terlihat.
type limiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// newIPLimiter membuat limiter dengan batas perMinute request per client.
func newIPLimiter(perMinute int) *ipLimiter {
	limit := rate.Limit(float64(perMinute) / 60)
	return &ipLimiter{
		perKey:   map[string]*limiterEntry{},
		overflow: rate.NewLimiter(limit, perMinute),
		limit:    limit,
		burst:    perMinute,
		maxKeys:  defaultMaxLimiterKeys,
	}
}

// allow melaporkan apakah client dengan key tertentu masih boleh mengirim
// request.
func (l *ipLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()

	if entry, ok := l.perKey[key]; ok {
		entry.lastSeen = now
		return entry.limiter.Allow()
	}
	if len(l.perKey) >= l.maxKeys && now.Sub(l.lastSweep) >= limiterSweepInterval {
		l.sweep(now)
	}
	if len(l.perKey) >= l.maxKeys {
		return l.overflow.Allow()
	}
	entry := &limiterEntry{limiter: rate.NewLimiter(l.limit, l.burst), lastSeen: now}
	l.perKey[key] = entry
	return entry.limiter.Allow()
}

// sweep membuang key yang tidak aktif. Dipanggil paling sering sekali per
// limiterSweepInterval, supaya biaya penyapuan tidak ditanggung setiap request.
func (l *ipLimiter) sweep(now time.Time) {
	for k, e := range l.perKey {
		if now.Sub(e.lastSeen) > limiterIdleTTL {
			delete(l.perKey, k)
		}
	}
	l.lastSweep = now
}

// limiterKey mengubah IP menjadi key rate limit. Alamat IPv6 dikelompokkan
// per prefix /64, karena satu pelanggan biasanya mendapat satu /64 dan bisa
// berganti alamat di dalamnya sesuka hati.
func limiterKey(ip string) string {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return ip
	}
	if addr.Is6() && !addr.Is4In6() {
		prefix, err := addr.Prefix(64)
		if err == nil {
			return prefix.String()
		}
	}
	return addr.Unmap().String()
}

// clientIPResolver menentukan IP client asli. Header x-forwarded-for hanya
// dipercaya bila koneksi datang dari proxy tepercaya, dan dibaca dari kanan
// sambil melewati alamat proxy, sama seperti di API HTTP.
type clientIPResolver struct {
	trusted []netip.Prefix
}

// newClientIPResolver membuat resolver dari daftar IP/CIDR proxy tepercaya.
func newClientIPResolver(trustedProxies []string) *clientIPResolver {
	r := &clientIPResolver{}
	for _, p := range trustedProxies {
		if prefix, err := netip.ParsePrefix(p); err == nil {
			r.trusted = append(r.trusted, prefix)
		} else if addr, err := netip.ParseAddr(p); err == nil {
			r.trusted = append(r.trusted, netip.PrefixFrom(addr, addr.BitLen()))
		}
	}
	return r
}

// isTrusted melaporkan apakah ip termasuk proxy tepercaya.
func (r *clientIPResolver) isTrusted(ip string) bool {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}
	for _, p := range r.trusted {
		if p.Contains(addr.Unmap()) {
			return true
		}
	}
	return false
}

// clientIP mengembalikan IP client untuk request gRPC ini.
func (r *clientIPResolver) clientIP(ctx context.Context) string {
	peerAddr := "unknown"
	if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
		peerAddr = p.Addr.String()
		if host, _, err := net.SplitHostPort(peerAddr); err == nil {
			peerAddr = host
		}
	}
	if !r.isTrusted(peerAddr) {
		return peerAddr
	}
	md, _ := metadata.FromIncomingContext(ctx)
	hops := strings.Split(strings.Join(md.Get("x-forwarded-for"), ","), ",")
	for i := len(hops) - 1; i >= 0; i-- {
		hop := strings.TrimSpace(hops[i])
		if _, err := netip.ParseAddr(hop); err != nil {
			continue
		}
		if !r.isTrusted(hop) {
			return hop
		}
	}
	return peerAddr
}

// rateLimitInterceptor menolak RPC dengan ResourceExhausted bila client
// melewati batas, setara dengan respons 429 di API HTTP.
func rateLimitInterceptor(perMinute int, trustedProxies []string) grpc.UnaryServerInterceptor {
	if perMinute <= 0 {
		return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			return handler(ctx, req)
		}
	}
	limiter := newIPLimiter(perMinute)
	resolver := newClientIPResolver(trustedProxies)
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if !limiter.allow(limiterKey(resolver.clientIP(ctx))) {
			return nil, status.Error(codes.ResourceExhausted, "too many requests, please retry later")
		}
		return handler(ctx, req)
	}
}
