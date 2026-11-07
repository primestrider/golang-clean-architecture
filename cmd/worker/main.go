// Command worker menjalankan pekerjaan latar belakang: consumer Kafka
// (dan, di tahap berikutnya, outbox relay serta scheduler) beserta server
// admin untuk health check dan metrics.
//
// Seperti binary web, worker melakukan graceful shutdown saat menerima
// SIGINT/SIGTERM: berhenti mengambil pesan baru, menunggu pesan yang
// sedang diproses selesai, lalu keluar.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/primestrider/golang-clean-architecture/internal/config"
	"github.com/primestrider/golang-clean-architecture/internal/delivery/admin"
	"github.com/primestrider/golang-clean-architecture/internal/delivery/messaging"
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	adminServer := admin.NewServer(fmt.Sprintf(":%d", cfg.Admin.WorkerPort), log, map[string]admin.Check{})
	adminErr := make(chan error, 1)
	go func() { adminErr <- adminServer.Start() }()

	// Setiap pekerjaan latar belakang didaftarkan ke WaitGroup supaya saat
	// shutdown kita bisa menunggu semuanya selesai.
	var wg sync.WaitGroup
	if cfg.Kafka.Enabled {
		consumers := map[string]messaging.ConsumerHandler{
			"users":     messaging.NewUserConsumer(log).Consume,
			"contacts":  messaging.NewContactConsumer(log).Consume,
			"addresses": messaging.NewAddressConsumer(log).Consume,
		}
		for topic, handler := range consumers {
			group, err := config.NewKafkaConsumerGroup(cfg)
			if err != nil {
				return err
			}
			wg.Go(func() { messaging.ConsumeTopic(ctx, group, topic, log, handler) })
		}
	} else {
		log.Info("kafka disabled, consumers not started")
	}

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

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-shutdownCtx.Done():
		log.Warn("shutdown timeout reached before background jobs finished")
	}

	err = errors.Join(runErr, adminServer.Shutdown(shutdownCtx))
	log.Info("shutdown complete")
	return err
}
