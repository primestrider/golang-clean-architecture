package grpc

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

func peerCtx(addr string, xff string) context.Context {
	host, port, _ := net.SplitHostPort(addr)
	tcp := &net.TCPAddr{IP: net.ParseIP(host)}
	_ = port
	ctx := peer.NewContext(context.Background(), &peer.Peer{Addr: tcp})
	if xff != "" {
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("x-forwarded-for", xff))
	}
	return ctx
}

func TestClientIP_UntrustedPeerIgnoresForwardedFor(t *testing.T) {
	resolver := newClientIPResolver(nil)

	ip := resolver.clientIP(peerCtx("198.51.100.7:5000", "203.0.113.9"))

	assert.Equal(t, "198.51.100.7", ip)
}

func TestClientIP_TrustedProxyUsesFirstUntrustedHopFromRight(t *testing.T) {
	resolver := newClientIPResolver([]string{"10.0.0.0/8"})

	ip := resolver.clientIP(peerCtx("10.0.0.5:5000", "1.1.1.1, 203.0.113.9, 10.0.0.4"))

	assert.Equal(t, "203.0.113.9", ip, "spoofed leftmost entries must be ignored")
}

func TestLimiterKey_GroupsIPv6ByPrefix(t *testing.T) {
	assert.Equal(t, limiterKey("2001:db8:1:2::1"), limiterKey("2001:db8:1:2:ffff::9"))
	assert.NotEqual(t, limiterKey("2001:db8:1:2::1"), limiterKey("2001:db8:1:3::1"))
	assert.Equal(t, "198.51.100.7", limiterKey("198.51.100.7"))
}

func TestIPLimiter_EvictsOnlyFullyRefilledEntries(t *testing.T) {
	l := newIPLimiter(1)
	l.maxKeys = 2

	assert.True(t, l.allow("a"))
	assert.True(t, l.allow("b"))
	// a dan b sedang di-throttle (budget habis). Mengeluarkan salah satunya
	// akan me-reset budget-nya, jadi client baru memakai bucket overflow.
	assert.True(t, l.allow("c"), "new client is served from the overflow bucket")
	assert.False(t, l.allow("d"), "overflow bucket is shared while the table is full of throttled clients")
	assert.False(t, l.allow("a"), "throttled clients are never reset by eviction")
	assert.Equal(t, 2, l.size())
}

func TestIPLimiter_EvictsRefilledEntryForNewClient(t *testing.T) {
	l := newIPLimiter(60) // 1 token per detik, burst 60
	l.maxKeys = 1
	now := time.Now()
	l.now = func() time.Time { return now }

	assert.True(t, l.allow("a")) // a tinggal 59 token, belum penuh
	now = now.Add(2 * time.Minute)

	assert.True(t, l.allow("b"), "a's bucket is full again, so evicting it changes nothing")
	assert.Equal(t, 1, l.size())
}
