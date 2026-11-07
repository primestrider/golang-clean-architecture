// Package admin menyediakan server HTTP internal untuk operasional:
// health check (liveness dan readiness) serta endpoint metrics.
//
// Server ini berjalan di port terpisah dari API publik. Port admin cukup
// dibuka untuk orchestrator (Kubernetes, Docker, load balancer) dan
// Prometheus, sehingga metrics dan detail kesehatan tidak terekspos ke
// internet.
package admin

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"
)

// Check adalah pemeriksaan kesehatan satu dependency (misalnya ping DB).
// Mengembalikan nil bila sehat.
type Check func(ctx context.Context) error

// checkTimeout membatasi lama satu pemeriksaan readiness, supaya dependency
// yang menggantung tidak membuat probe ikut menggantung.
const checkTimeout = 2 * time.Second

// Server adalah server admin.
type Server struct {
	log    *slog.Logger
	checks map[string]Check
	ready  atomic.Bool
	mux    *http.ServeMux
	srv    *http.Server
}

// NewServer membuat server admin di alamat addr dengan daftar pemeriksaan
// readiness. Server langsung berstatus ready.
func NewServer(addr string, log *slog.Logger, checks map[string]Check) *Server {
	s := &Server{log: log, checks: checks, mux: http.NewServeMux()}
	s.ready.Store(true)
	s.mux.HandleFunc("GET /health/live", s.live)
	s.mux.HandleFunc("GET /health/ready", s.readiness)
	s.srv = &http.Server{Addr: addr, Handler: s.mux, ReadHeaderTimeout: 5 * time.Second}
	return s
}

// Handle memasang handler tambahan, misalnya /metrics.
func (s *Server) Handle(pattern string, h http.Handler) {
	s.mux.Handle(pattern, h)
}

// Handler mengembalikan router HTTP server ini (berguna untuk test).
func (s *Server) Handler() http.Handler { return s.mux }

// SetReady mengubah status readiness. Saat shutdown dimulai, status diubah
// ke false supaya load balancer berhenti mengirim traffic baru sebelum
// server benar-benar berhenti.
func (s *Server) SetReady(ready bool) { s.ready.Store(ready) }

// Start menjalankan server dan memblokir sampai server berhenti.
// http.ErrServerClosed (berhenti normal karena Shutdown) tidak dianggap error.
func (s *Server) Start() error {
	s.log.Info("admin server listening", "addr", s.srv.Addr)
	if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Shutdown menghentikan server dengan menunggu request yang sedang berjalan.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

// live menjawab liveness probe: selama proses bisa menjawab, ia hidup.
// Liveness sengaja tidak mengecek dependency. Jika database mati, me-restart
// aplikasi tidak menolong dan hanya menambah kekacauan.
func (s *Server) live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

// readiness menjawab readiness probe: apakah aplikasi siap menerima traffic.
func (s *Server) readiness(w http.ResponseWriter, r *http.Request) {
	if !s.ready.Load() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "shutting_down"})
		return
	}

	// Pemanggil hanya menerima nama check dan status generik. Detail error
	// (bisa berisi host, port, atau nama database) hanya dicatat di log.
	failed := map[string]string{}
	for name, check := range s.checks {
		ctx, cancel := context.WithTimeout(r.Context(), checkTimeout)
		err := check(ctx)
		cancel()
		if err != nil {
			failed[name] = "unavailable"
			s.log.Warn("readiness check failed", "check", name, "error", err)
		}
	}
	if len(failed) > 0 {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "unavailable", "checks": failed})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

// writeJSON menulis body JSON dengan status yang diberikan.
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
