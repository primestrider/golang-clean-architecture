package admin

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func get(t *testing.T, s *Server, path string) (int, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return rec.Code, body
}

func newServer(checks map[string]Check) *Server {
	return NewServer(":0", slog.New(slog.DiscardHandler), checks)
}

func TestLive_AlwaysOK(t *testing.T) {
	s := newServer(map[string]Check{"db": func(context.Context) error { return errors.New("down") }})

	code, body := get(t, s, "/health/live")

	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "ok", body["status"])
}

func TestReady_OKWhenChecksPass(t *testing.T) {
	s := newServer(map[string]Check{"db": func(context.Context) error { return nil }})

	code, _ := get(t, s, "/health/ready")

	assert.Equal(t, http.StatusOK, code)
}

func TestReady_FailsWhenCheckFails(t *testing.T) {
	s := newServer(map[string]Check{"db": func(context.Context) error { return errors.New("connection refused") }})

	code, body := get(t, s, "/health/ready")

	assert.Equal(t, http.StatusServiceUnavailable, code)
	assert.Equal(t, "unavailable", body["status"])
	// Detail error hanya masuk log, tidak dikirim ke pemanggil endpoint.
	assert.Equal(t, "unavailable", body["checks"].(map[string]any)["db"])
}

func TestReady_503AfterSetReadyFalse(t *testing.T) {
	s := newServer(map[string]Check{"db": func(context.Context) error { return nil }})

	s.SetReady(false)
	code, body := get(t, s, "/health/ready")

	assert.Equal(t, http.StatusServiceUnavailable, code)
	assert.Equal(t, "shutting_down", body["status"])
}

func TestHandle_MountsExtraHandler(t *testing.T) {
	s := newServer(nil)
	s.Handle("/metrics", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"metrics"}`))
	}))

	code, body := get(t, s, "/metrics")

	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "metrics", body["status"])
}
