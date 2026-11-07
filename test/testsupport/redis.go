package testsupport

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/redis"
)

// StartRedis menjalankan container Redis untuk satu test dan mengembalikan
// alamatnya (host:port). Container dimatikan otomatis saat test selesai.
func StartRedis(t testing.TB) string {
	t.Helper()
	ctx := context.Background()
	container, err := redis.Run(ctx, "redis:7.4-alpine")
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })

	uri, err := container.ConnectionString(ctx)
	require.NoError(t, err)
	return strings.TrimPrefix(uri, "redis://")
}
