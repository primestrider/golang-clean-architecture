package grpc

import (
	"context"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/primestrider/golang-clean-architecture/internal/observability"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/test/bufconn"
)

// TestShutdown_DoesNotHangOnOpenStreams: stream yang tidak pernah selesai
// (misalnya health Watch dari load balancer) tidak boleh menahan shutdown
// melewati batas waktunya.
func TestShutdown_DoesNotHangOnOpenStreams(t *testing.T) {
	server := NewServer(ServerOptions{
		Log:     slog.New(slog.DiscardHandler),
		Metrics: observability.NewMetrics(prometheus.NewRegistry()),
	})
	listener := bufconn.Listen(1 << 20)
	go func() { _ = server.Serve(listener) }()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()
	stream, err := healthpb.NewHealthClient(conn).Watch(context.Background(), &healthpb.HealthCheckRequest{})
	require.NoError(t, err)
	_, err = stream.Recv()
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	Shutdown(ctx, server)

	assert.Less(t, time.Since(start), 2*time.Second)
}
