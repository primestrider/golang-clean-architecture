package grpc

import (
	"context"
	"net"
	"sync"
	"time"

	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
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

// ipLimiter adalah rate limiter token bucket per IP client, disimpan di
// memori proses ini. Untuk beberapa instance, batasnya berlaku per instance.
type ipLimiter struct {
	mu       sync.Mutex
	perIP    map[string]*rate.Limiter
	limit    rate.Limit
	burst    int
	lastSeen map[string]time.Time
}

// newIPLimiter membuat limiter dengan batas perMinute request per IP.
func newIPLimiter(perMinute int) *ipLimiter {
	return &ipLimiter{
		perIP:    map[string]*rate.Limiter{},
		lastSeen: map[string]time.Time{},
		limit:    rate.Limit(float64(perMinute) / 60),
		burst:    perMinute,
	}
}

// allow melaporkan apakah IP masih boleh mengirim request. Entri IP yang
// lama tidak aktif dibuang supaya map tidak tumbuh tanpa batas.
func (l *ipLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if len(l.perIP) > 10000 {
		for k, seen := range l.lastSeen {
			if now.Sub(seen) > 10*time.Minute {
				delete(l.perIP, k)
				delete(l.lastSeen, k)
			}
		}
	}
	lim, ok := l.perIP[ip]
	if !ok {
		lim = rate.NewLimiter(l.limit, l.burst)
		l.perIP[ip] = lim
	}
	l.lastSeen[ip] = now
	return lim.Allow()
}

// rateLimitInterceptor menolak RPC dengan ResourceExhausted bila IP client
// melewati batas, setara dengan respons 429 di API HTTP.
func rateLimitInterceptor(perMinute int) grpc.UnaryServerInterceptor {
	if perMinute <= 0 {
		return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			return handler(ctx, req)
		}
	}
	limiter := newIPLimiter(perMinute)
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if !limiter.allow(peerIP(ctx)) {
			return nil, status.Error(codes.ResourceExhausted, "too many requests, please retry later")
		}
		return handler(ctx, req)
	}
}

// peerIP mengambil IP client dari koneksi gRPC.
func peerIP(ctx context.Context) string {
	p, ok := peer.FromContext(ctx)
	if !ok || p.Addr == nil {
		return "unknown"
	}
	host, _, err := net.SplitHostPort(p.Addr.String())
	if err != nil {
		return p.Addr.String()
	}
	return host
}
