// Command worker menjalankan pekerjaan latar belakang: outbox relay,
// consumer Kafka, dan (di tahap berikutnya) scheduler, beserta server admin
// untuk health check dan metrics.
//
// Seperti binary web, worker melakukan graceful shutdown saat menerima
// SIGINT/SIGTERM: berhenti mengambil pekerjaan baru, menunggu pekerjaan yang
// sedang berjalan selesai, lalu menutup koneksi.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/primestrider/golang-clean-architecture/internal/config"
	"github.com/primestrider/golang-clean-architecture/internal/delivery/admin"
	"github.com/primestrider/golang-clean-architecture/internal/observability"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	if err := run(); err != nil {
		slog.Error("worker stopped with error", "error", err)
		os.Exit(1)
	}
}

// run berisi seluruh logika main; lihat cmd/web untuk alasan pemisahannya.
func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := config.NewLogger(cfg).With("component", "worker")

	db, err := config.NewDatabase(cfg, log)
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}

	registry := prometheus.NewRegistry()
	metrics := observability.NewMetrics(registry)
	metrics.Register(collectors.NewDBStatsCollector(sqlDB, cfg.Database.Name))

	worker, err := config.BootstrapWorker(&config.WorkerDeps{Config: cfg, Log: log, DB: db, Metrics: metrics})
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	adminServer := admin.NewServer(fmt.Sprintf(":%d", cfg.Admin.WorkerPort), log, map[string]admin.Check{
		"db": sqlDB.PingContext,
	})
	adminServer.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	adminErr := make(chan error, 1)
	go func() { adminErr <- adminServer.Start() }()

	jobsDone := make(chan struct{})
	go func() { worker.Run(ctx); close(jobsDone) }()

	var runErr error
	select {
	case <-ctx.Done():
		log.Info("shutdown signal received")
	case runErr = <-adminErr:
		log.Error("admin server stopped unexpectedly", "error", runErr)
		stop()
	}

	adminServer.SetReady(false)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Shutdown.Timeout)
	defer cancel()

	select {
	case <-jobsDone:
	case <-shutdownCtx.Done():
		log.Warn("shutdown timeout reached before background jobs finished")
	}

	err = errors.Join(runErr, adminServer.Shutdown(shutdownCtx), worker.Close(), sqlDB.Close())
	log.Info("shutdown complete")
	return err
}
