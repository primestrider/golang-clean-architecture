package testsupport

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Mailpit adalah server SMTP palsu untuk test: menerima email tanpa
// mengirimkannya dan menyediakan API HTTP untuk memeriksa isinya.
type Mailpit struct {
	Host     string
	SMTPPort int
	apiURL   string
}

// StartMailpit menjalankan container Mailpit untuk satu test.
func StartMailpit(t testing.TB) *Mailpit {
	t.Helper()
	ctx := context.Background()
	container, err := testcontainers.Run(ctx, "axllent/mailpit:v1.27",
		testcontainers.WithExposedPorts("1025/tcp", "8025/tcp"),
		testcontainers.WithWaitStrategy(wait.ForHTTP("/api/v1/info").WithPort("8025/tcp")),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })

	host, err := container.Host(ctx)
	require.NoError(t, err)
	smtp, err := container.MappedPort(ctx, "1025/tcp")
	require.NoError(t, err)
	api, err := container.PortEndpoint(ctx, "8025/tcp", "http")
	require.NoError(t, err)
	port, _ := strconv.Atoi(smtp.Port())
	return &Mailpit{Host: host, SMTPPort: port, apiURL: api}
}

// MessageCount mengembalikan jumlah email yang sudah diterima.
func (m *Mailpit) MessageCount(t testing.TB) int {
	t.Helper()
	resp, err := http.Get(m.apiURL + "/api/v1/messages")
	require.NoError(t, err)
	defer resp.Body.Close()
	var body struct {
		Total int `json:"total"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	return body.Total
}
