// Command web menjalankan API HTTP beserta server admin (health check dan
// metrics).
//
// Siklus hidupnya: muat config, buat dependency, jalankan server, lalu
// tunggu sinyal SIGINT/SIGTERM. Saat sinyal datang, aplikasi melakukan
// graceful shutdown: berhenti menerima request baru, menunggu request yang
// sedang berjalan selesai, lalu menutup koneksi ke dependency.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v3"
	"github.com/primestrider/golang-clean-architecture/internal/config"
	"github.com/primestrider/golang-clean-architecture/internal/delivery/admin"
	"github.com/primestrider/golang-clean-architecture/internal/observability"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	if err := run(); err != nil {
		slog.Error("web stopped with error", "error", err)
		os.Exit(1)
	}
}

// run berisi seluruh logika main. Dipisah supaya error cukup dikembalikan
// dan defer tetap berjalan (os.Exit di tengah fungsi akan melewati defer).
func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := config.NewLogger(cfg)

	shutdownTracing, err := config.NewTracerProvider(context.Background(), cfg)
	if err != nil {
		return err
	}

	db, err := config.NewDatabase(cfg, log)
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}

	// Registry Prometheus milik aplikasi (bukan registry global), supaya
	// metrik yang diekspos hanya yang memang didaftarkan aplikasi ini.
	registry := prometheus.NewRegistry()
	metrics := observability.NewMetrics(registry)
	metrics.Register(collectors.NewDBStatsCollector(sqlDB, cfg.Database.Name))

	redisClient, err := config.NewRedis(cfg)
	if err != nil {
		return err
	}

	app := config.NewFiber(cfg, log)
	config.BootstrapWeb(&config.WebDeps{Config: cfg, Log: log, DB: db, App: app, Metrics: metrics, Redis: redisClient})

	checks := map[string]admin.Check{"db": sqlDB.PingContext}
	if redisClient != nil {
		checks["redis"] = func(ctx context.Context) error { return redisClient.Ping(ctx).Err() }
	}
	adminServer := admin.NewServer(fmt.Sprintf(":%d", cfg.Admin.Port), log, checks)
	adminServer.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	// ctx dibatalkan saat menerima SIGINT (Ctrl+C) atau SIGTERM (dikirim
	// Docker/Kubernetes ketika container dihentikan).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverErr := make(chan error, 2)
	go func() { serverErr <- adminServer.Start() }()
	go func() {
		addr := fmt.Sprintf(":%d", cfg.Server.Port)
		log.Info("http server listening", "addr", addr)
		serverErr <- app.Listen(addr, fiber.ListenConfig{DisableStartupMessage: true})
	}()

	var runErr error
	select {
	case <-ctx.Done():
		log.Info("shutdown signal received")
	case runErr = <-serverErr:
		log.Error("server stopped unexpectedly", "error", runErr)
	}

	// Urutan shutdown penting:
	//  1. tandai not-ready supaya load balancer berhenti mengirim traffic,
	//  2. hentikan server dan tunggu request yang sedang berjalan,
	//  3. baru tutup dependency yang dipakai request tersebut.
	adminServer.SetReady(false)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Shutdown.Timeout)
	defer cancel()

	errs := []error{runErr}
	errs = append(errs, app.ShutdownWithContext(shutdownCtx))
	errs = append(errs, adminServer.Shutdown(shutdownCtx))
	if redisClient != nil {
		errs = append(errs, redisClient.Close())
	}
	errs = append(errs, sqlDB.Close())
	errs = append(errs, shutdownTracing(shutdownCtx))

	log.Info("shutdown complete")
	return errors.Join(errs...)
}
