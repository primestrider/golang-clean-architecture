package config

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/primestrider/golang-clean-architecture/internal/entity"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/go-co-op/gocron/v2"
	"github.com/primestrider/golang-clean-architecture/internal/delivery/messaging"
	"github.com/primestrider/golang-clean-architecture/internal/observability"
	"github.com/primestrider/golang-clean-architecture/internal/outbox"
	"github.com/primestrider/golang-clean-architecture/internal/repository"
	"github.com/primestrider/golang-clean-architecture/internal/scheduler"
	"github.com/primestrider/golang-clean-architecture/internal/usecase"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// WorkerDeps adalah dependency infrastruktur untuk binary worker.
type WorkerDeps struct {
	Config *Config
	Log    *slog.Logger
	DB     *gorm.DB
	// Metrics mencatat metrik outbox dan consumer; wajib diisi.
	Metrics *observability.Metrics
	// Redis opsional (nil bila dimatikan): dipakai sebagai lock scheduler
	// supaya job terjadwal hanya berjalan di satu replica.
	Redis *redis.Client
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
	if err := registerHandlers(registry, deps); err != nil {
		return nil, err
	}

	outboxRepo := repository.NewOutboxRepository(deps.DB)
	registerOutboxGauges(deps.Metrics, outboxRepo)

	if err := w.addScheduler(deps, outboxRepo); err != nil {
		return nil, err
	}
	locker := outbox.NewMySQLLocker(deps.DB, "outbox_relay")

	if !cfg.Kafka.Enabled {
		deps.Log.Info("kafka disabled, outbox events are dispatched to local handlers")
		relay := outbox.NewRelay(outboxRepo, messaging.NewLocalDispatcher(registry), locker, outboxOptions(cfg.Outbox), deps.Log)
		relay.OnBatch = relayMetrics(deps.Metrics)
		w.jobs = append(w.jobs, job{name: "outbox-relay", run: relay.Run})
		return w, nil
	}

	producer, err := NewKafkaProducer(cfg, deps.Log)
	if err != nil {
		return nil, err
	}
	w.closers = append(w.closers, producer.Close)

	relay := outbox.NewRelay(outboxRepo, outbox.NewKafkaDispatcher(producer), locker, outboxOptions(cfg.Outbox), deps.Log)
	relay.OnBatch = relayMetrics(deps.Metrics)
	w.jobs = append(w.jobs, job{name: "outbox-relay", run: relay.Run})

	group, err := NewKafkaConsumerGroup(cfg)
	if err != nil {
		return nil, errors.Join(err, w.Close())
	}
	processor := messaging.NewProcessor(registry, producer, cfg.Consumer.MaxRetries, cfg.Consumer.InitialBackoff, deps.Log).
		WithMetrics(deps.Metrics)
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
func registerHandlers(registry *messaging.Registry, deps *WorkerDeps) error {
	registry.Register("event-logger", messaging.AnyEvent, messaging.NewLoggingHandler(deps.Log))

	mailer, err := NewMailer(deps.Config, deps.Log)
	if err != nil {
		return err
	}
	notifications := usecase.NewNotificationUseCase(deps.Log, mailer)
	registry.Register("welcome-email", "user.registered", notifications.SendWelcome)
	return nil
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

// relayMetrics mengembalikan callback relay yang mencatat jumlah event
// terkirim dan gagal.
func relayMetrics(m *observability.Metrics) func(published, failed int) {
	return func(published, failed int) {
		m.OutboxPublished.Add(float64(published))
		m.OutboxPublishFailures.Add(float64(failed))
	}
}

// registerOutboxGauges mendaftarkan gauge jumlah event outbox per status.
// Nilainya dihitung saat Prometheus melakukan scrape.
func registerOutboxGauges(m *observability.Metrics, repo *repository.OutboxRepository) {
	count := func(status string) func() float64 {
		return func() float64 {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			counts, err := repo.CountByStatus(ctx)
			if err != nil {
				return -1
			}
			return float64(counts[status])
		}
	}
	m.Register(
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "outbox_pending", Help: "Jumlah event outbox yang menunggu dikirim."}, count(entity.OutboxPending)),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "outbox_failed", Help: "Jumlah event outbox yang diparkir karena gagal terus."}, count(entity.OutboxFailed)),
	)
}

// addScheduler mendaftarkan scheduler job pembersihan sebagai pekerjaan
// worker. Tanpa Redis, scheduler berjalan tanpa lock (aman bila hanya ada
// satu replica worker).
func (w *Worker) addScheduler(deps *WorkerDeps, outboxRepo *repository.OutboxRepository) error {
	var locker gocron.Locker
	if deps.Redis != nil {
		locker = scheduler.NewRedisLocker(deps.Redis, 10*time.Minute)
	} else {
		deps.Log.Info("redis disabled, scheduler runs without distributed lock (single worker replica assumed)")
	}
	s, err := scheduler.New(scheduler.Options{
		TokenCleanupCron:  deps.Config.Scheduler.TokenCleanupCron,
		OutboxCleanupCron: deps.Config.Scheduler.OutboxCleanupCron,
	}, locker, deps.Log, scheduler.Jobs{
		RefreshTokens: repository.NewRefreshTokenRepository(deps.DB),
		Outbox:        outboxRepo,
		Processed:     repository.NewProcessedEventRepository(deps.DB),
		Now:           time.Now,
	})
	if err != nil {
		return err
	}
	w.jobs = append(w.jobs, job{name: "scheduler", run: func(ctx context.Context) error {
		s.Start()
		<-ctx.Done()
		return s.Shutdown()
	}})
	return nil
}
