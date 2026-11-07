// Package outbox mengirim event dari tabel outbox ke tujuannya (Kafka, atau
// handler lokal bila Kafka dimatikan).
//
// Alurnya: use case menulis event ke tabel outbox di dalam transaksi bisnis.
// Relay di worker membaca event pending secara berkala lalu mengirimnya lewat
// Dispatcher. Event ditandai published hanya setelah pengiriman berhasil,
// sehingga jaminannya at-least-once: event bisa terkirim lebih dari sekali
// (misalnya worker mati tepat setelah mengirim), tetapi tidak pernah hilang.
// Karena itu consumer harus idempoten.
package outbox

import (
	"context"
	"log/slog"
	"time"

	"github.com/primestrider/golang-clean-architecture/internal/entity"
)

// Repository adalah akses tabel outbox yang dibutuhkan relay.
type Repository interface {
	FetchPending(ctx context.Context, limit int) ([]entity.OutboxEvent, error)
	MarkPublished(ctx context.Context, id string, at int64) error
	MarkFailedAttempt(ctx context.Context, id, errMsg string, maxAttempts int) error
}

// Dispatcher mengirim satu event ke tujuannya.
type Dispatcher interface {
	Dispatch(ctx context.Context, e entity.OutboxEvent) error
}

// Locker memastikan hanya satu relay yang aktif di antara banyak replica
// worker. Dengan satu relay aktif, urutan event per aggregate terjaga.
type Locker interface {
	// TryLock mencoba mengambil (atau memastikan masih memegang) lock tanpa
	// menunggu. Mengembalikan true bila lock dipegang relay ini.
	TryLock(ctx context.Context) (bool, error)
	Unlock(ctx context.Context) error
}

// Options mengatur ritme relay.
type Options struct {
	// PollInterval adalah jeda antar pemeriksaan outbox.
	PollInterval time.Duration
	// BatchSize adalah jumlah maksimum event per putaran.
	BatchSize int
	// MaxAttempts adalah batas percobaan sebelum event diparkir (failed).
	MaxAttempts int
}

// Relay membaca event pending dari outbox dan mengirimkannya.
type Relay struct {
	repo       Repository
	dispatcher Dispatcher
	locker     Locker
	cfg        Options
	log        *slog.Logger
	now        func() time.Time
	// OnBatch dipanggil setelah setiap batch (opsional), misalnya untuk metrik.
	OnBatch func(published int, failed int)
}

// NewRelay membuat Relay.
func NewRelay(repo Repository, dispatcher Dispatcher, locker Locker, cfg Options, log *slog.Logger) *Relay {
	return &Relay{repo: repo, dispatcher: dispatcher, locker: locker, cfg: cfg, log: log.With("component", "outbox-relay"), now: time.Now}
}

// Run menjalankan relay sampai ctx dibatalkan. Di setiap putaran, relay
// memastikan dirinya pemegang lock; replica yang tidak memegang lock hanya
// menunggu dan siap mengambil alih bila pemegang lock berhenti.
func (r *Relay) Run(ctx context.Context) error {
	ticker := time.NewTicker(r.cfg.PollInterval)
	defer ticker.Stop()
	defer func() {
		// Lock dilepas memakai context baru karena ctx sudah dibatalkan.
		if err := r.locker.Unlock(context.WithoutCancel(ctx)); err != nil {
			r.log.Warn("release outbox lock failed", "error", err)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}

		leader, err := r.locker.TryLock(ctx)
		if err != nil {
			r.log.WarnContext(ctx, "acquire outbox lock failed", "error", err)
			continue
		}
		if !leader {
			continue
		}
		// Kosongkan antrean selagi ada pekerjaan, lalu kembali menunggu tick.
		for ctx.Err() == nil {
			published, err := r.ProcessBatch(ctx)
			if err != nil {
				r.log.ErrorContext(ctx, "process outbox batch failed", "error", err)
				break
			}
			if published < r.cfg.BatchSize {
				break
			}
		}
	}
}

// ProcessBatch mengirim satu batch event pending dan mengembalikan jumlah
// yang berhasil dikirim.
//
// Bila satu event gagal, event berikutnya dengan aggregate yang sama di
// batch ini tidak dikirim, supaya urutan per aggregate tidak terbalik.
// Event itu dicoba lagi di putaran berikutnya; setelah max_attempts kali
// gagal, repository menandainya failed (diparkir).
func (r *Relay) ProcessBatch(ctx context.Context) (int, error) {
	events, err := r.repo.FetchPending(ctx, r.cfg.BatchSize)
	if err != nil {
		return 0, err
	}

	blocked := map[string]bool{}
	published, failed := 0, 0
	for _, e := range events {
		if blocked[e.AggregateID] {
			continue
		}
		if err := r.dispatcher.Dispatch(ctx, e); err != nil {
			blocked[e.AggregateID] = true
			failed++
			r.log.WarnContext(ctx, "dispatch outbox event failed",
				"event_id", e.ID, "event_type", e.EventType, "attempt", e.Attempts+1, "error", err)
			if err := r.repo.MarkFailedAttempt(ctx, e.ID, err.Error(), r.cfg.MaxAttempts); err != nil {
				return published, err
			}
			continue
		}
		if err := r.repo.MarkPublished(ctx, e.ID, r.now().UnixMilli()); err != nil {
			return published, err
		}
		published++
	}
	if r.OnBatch != nil {
		r.OnBatch(published, failed)
	}
	return published, nil
}
