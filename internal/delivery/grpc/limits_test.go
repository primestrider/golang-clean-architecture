package grpc

import (
	"context"
	"net"
	"testing"

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

func TestIPLimiter_EvictsLeastRecentlyUsedWhenFull(t *testing.T) {
	l := newIPLimiter(1)
	l.maxKeys = 2

	assert.True(t, l.allow("a"))
	assert.True(t, l.allow("b"))
	assert.False(t, l.allow("a"), "a has used its budget; this also marks a as most recent")
	assert.True(t, l.allow("c"), "new client gets its own bucket by evicting the least recently used (b)")
	assert.False(t, l.allow("a"), "a keeps its exhausted bucket: eviction must not reset recently active clients")
	assert.Equal(t, 2, l.size())
}
