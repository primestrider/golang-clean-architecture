package grpc

import (
	"container/list"
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

// defaultMaxLimiterKeys adalah jumlah maksimum client yang dilacak limiter.
const defaultMaxLimiterKeys = 10000

// ipLimiter adalah rate limiter token bucket per client, disimpan di memori
// proses ini (untuk beberapa instance, batasnya berlaku per instance).
//
// Jumlah client yang dilacak dibatasi (maxKeys) dengan LRU, tetapi entri
// hanya dikeluarkan bila bucket-nya sudah terisi penuh kembali. Mengeluarkan
// entri seperti itu tidak mengubah apa pun, karena bucket barunya juga penuh.
// Client yang sedang di-throttle tidak pernah di-reset. Bila semua entri
// sedang di-throttle (tanda serangan dari banyak alamat), client baru
// memakai satu bucket overflow bersama. Semua operasi O(1). Serangan dari
// puluhan ribu IP sebaiknya ditahan di edge (WAF atau load balancer).
type ipLimiter struct {
	mu       sync.Mutex
	entries  map[string]*list.Element
	order    *list.List // depan = paling baru dipakai
	overflow *rate.Limiter
	limit    rate.Limit
	burst    int
	maxKeys  int
	// now disuntikkan supaya pengisian ulang token bisa diuji.
	now func() time.Time
}

// lruEntry adalah satu client di dalam LRU.
type lruEntry struct {
	key     string
	limiter *rate.Limiter
}

// newIPLimiter membuat limiter dengan batas perMinute request per client.
func newIPLimiter(perMinute int) *ipLimiter {
	limit := rate.Limit(float64(perMinute) / 60)
	return &ipLimiter{
		entries:  map[string]*list.Element{},
		order:    list.New(),
		overflow: rate.NewLimiter(limit, perMinute),
		limit:    limit,
		burst:    perMinute,
		maxKeys:  defaultMaxLimiterKeys,
		now:      time.Now,
	}
}

// allow melaporkan apakah client dengan key tertentu masih boleh mengirim
// request.
func (l *ipLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()

	if el, ok := l.entries[key]; ok {
		l.order.MoveToFront(el)
		entry, _ := el.Value.(*lruEntry)
		return entry.limiter.AllowN(now, 1)
	}
	if l.order.Len() >= l.maxKeys {
		oldest := l.order.Back()
		entry, _ := oldest.Value.(*lruEntry)
		if entry.limiter.TokensAt(now) < float64(l.burst) {
			// Entri terlama masih di-throttle: jangan di-reset.
			return l.overflow.AllowN(now, 1)
		}
		l.order.Remove(oldest)
		delete(l.entries, entry.key)
	}
	entry := &lruEntry{key: key, limiter: rate.NewLimiter(l.limit, l.burst)}
	l.entries[key] = l.order.PushFront(entry)
	return entry.limiter.AllowN(now, 1)
}

// size mengembalikan jumlah client yang sedang dilacak.
func (l *ipLimiter) size() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.order.Len()
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
