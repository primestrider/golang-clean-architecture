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
//   - registry handler event (idempoten lewat tabel processed_events);
//   - outbox relay yang selalu berjalan. Bila Kafka aktif, relay mengirim
//     event ke Kafka dan consumer Kafka meneruskannya ke registry. Bila Kafka
//     mati, relay langsung memanggil registry di proses ini (mode minimal).
func BootstrapWorker(deps *WorkerDeps) (*Worker, error) {
	w := &Worker{log: deps.Log}
	cfg := deps.Config

	registry := messaging.NewRegistry(repository.NewTransactor(deps.DB),
		repository.NewProcessedEventRepository(deps.DB), deps.Log)
	registerHandlers(registry, deps)

	outboxRepo := repository.NewOutboxRepository(deps.DB)
	locker := outbox.NewMySQLLocker(deps.DB, "outbox_relay")

	if !cfg.Kafka.Enabled {
		deps.Log.Info("kafka disabled, outbox events are dispatched to local handlers")
		relay := outbox.NewRelay(outboxRepo, messaging.NewLocalDispatcher(registry), locker, outboxOptions(cfg.Outbox), deps.Log)
		w.jobs = append(w.jobs, job{name: "outbox-relay", run: relay.Run})
		return w, nil
	}

	producer, err := NewKafkaProducer(cfg, deps.Log)
	if err != nil {
		return nil, err
	}
	w.closers = append(w.closers, producer.Close)

	relay := outbox.NewRelay(outboxRepo, outbox.NewKafkaDispatcher(producer), locker, outboxOptions(cfg.Outbox), deps.Log)
	w.jobs = append(w.jobs, job{name: "outbox-relay", run: relay.Run})

	group, err := NewKafkaConsumerGroup(cfg)
	if err != nil {
		return nil, errors.Join(err, w.Close())
	}
	processor := messaging.NewProcessor(registry, producer, cfg.Consumer.MaxRetries, cfg.Consumer.InitialBackoff, deps.Log)
	topics := registry.Topics()
	w.jobs = append(w.jobs, job{name: "kafka-consumer", run: func(ctx context.Context) error {
		messaging.ConsumeTopics(ctx, group, topics, processor, deps.Log)
		return nil
	}})
	return w, nil
}

// registerHandlers mendaftarkan semua handler event. Tambahkan handler baru
// di sini: nama consumer harus unik dan tidak boleh diganti setelah dipakai,
// karena menjadi kunci idempotency.
func registerHandlers(registry *messaging.Registry, deps *WorkerDeps) {
	registry.Register("event-logger", messaging.AnyEvent, messaging.NewLoggingHandler(deps.Log))
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
