package config

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/primestrider/golang-clean-architecture/internal/delivery/messaging"
	"github.com/primestrider/golang-clean-architecture/internal/outbox"
	"github.com/primestrider/golang-clean-architecture/internal/repository"
	"gorm.io/gorm"
)

// WorkerDeps adalah dependency infrastruktur untuk binary worker.
type WorkerDeps struct {
	Config *Config
	Log    *slog.Logger
	DB     *gorm.DB
}

// job adalah satu pekerjaan latar belakang yang berjalan sampai ctx batal.
type job struct {
	name string
	run  func(ctx context.Context) error
}

// Worker menampung semua pekerjaan latar belakang beserta sumber daya yang
// harus ditutup saat berhenti.
type Worker struct {
	log     *slog.Logger
	jobs    []job
	closers []func() error
}

// BootstrapWorker merangkai pekerjaan worker:
//   - outbox relay: mengirim event dari tabel outbox ke Kafka;
//   - consumer Kafka untuk setiap topik.
//
// Bila Kafka dimatikan, relay dan consumer Kafka tidak dijalankan.
func BootstrapWorker(deps *WorkerDeps) (*Worker, error) {
	w := &Worker{log: deps.Log}
	cfg := deps.Config

	if !cfg.Kafka.Enabled {
		deps.Log.Info("kafka disabled, outbox relay and kafka consumers not started")
		return w, nil
	}

	producer, err := NewKafkaProducer(cfg, deps.Log)
	if err != nil {
		return nil, err
	}
	w.closers = append(w.closers, producer.Close)

	relay := outbox.NewRelay(repository.NewOutboxRepository(deps.DB), outbox.NewKafkaDispatcher(producer),
		outbox.NewMySQLLocker(deps.DB, "outbox_relay"), outboxOptions(cfg.Outbox), deps.Log)
	w.jobs = append(w.jobs, job{name: "outbox-relay", run: relay.Run})

	handler := messaging.NewLoggingHandler(deps.Log)
	for _, topic := range []string{"users", "contacts", "addresses"} {
		group, err := NewKafkaConsumerGroup(cfg)
		if err != nil {
			return nil, errors.Join(err, w.Close())
		}
		w.jobs = append(w.jobs, job{name: "consumer-" + topic, run: func(ctx context.Context) error {
			messaging.ConsumeTopic(ctx, group, topic, deps.Log, handler)
			return nil
		}})
	}
	return w, nil
}

// Run menjalankan semua pekerjaan dan memblokir sampai semuanya berhenti
// (biasanya karena ctx dibatalkan saat shutdown).
func (w *Worker) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for _, j := range w.jobs {
		wg.Go(func() {
			w.log.Info("background job started", "job", j.name)
			if err := j.run(ctx); err != nil {
				w.log.Error("background job stopped with error", "job", j.name, "error", err)
			}
			w.log.Info("background job stopped", "job", j.name)
		})
	}
	wg.Wait()
}

// Close menutup sumber daya milik worker (producer Kafka, dan sebagainya).
func (w *Worker) Close() error {
	var errs []error
	for _, c := range w.closers {
		errs = append(errs, c())
	}
	return errors.Join(errs...)
}

// outboxOptions memetakan konfigurasi outbox ke opsi relay.
func outboxOptions(c OutboxConfig) outbox.Options {
	return outbox.Options{PollInterval: c.PollInterval, BatchSize: c.BatchSize, MaxAttempts: c.MaxAttempts}
}
