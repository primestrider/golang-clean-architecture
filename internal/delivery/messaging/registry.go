package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/primestrider/golang-clean-architecture/internal/entity"
	"github.com/primestrider/golang-clean-architecture/internal/model"
	"github.com/primestrider/golang-clean-architecture/internal/observability"
	"github.com/primestrider/golang-clean-architecture/internal/outbox"
	"github.com/primestrider/golang-clean-architecture/internal/usecase"
)

// AnyEvent dipakai saat Register untuk menerima semua jenis event.
const AnyEvent = "*"

// Handler memproses satu event domain.
type Handler func(ctx context.Context, e model.Event) error

// ProcessedEventRepository mencatat event yang sudah diproses tiap consumer.
type ProcessedEventRepository interface {
	// TryMark mencatat (consumer, eventID). Mengembalikan false bila sudah
	// pernah dicatat, artinya event ini duplikat.
	TryMark(ctx context.Context, consumer, eventID string) (bool, error)
}

// registration adalah satu handler terdaftar.
type registration struct {
	consumer  string
	eventType string
	handler   Handler
}

// Registry menyimpan daftar handler per jenis event dan menjalankannya
// secara idempoten.
//
// Pengiriman event bersifat at-least-once, jadi event yang sama bisa datang
// lebih dari sekali. Registry mencatat setiap (consumer, event_id) yang sudah
// diproses di tabel processed_events, di transaksi yang sama dengan efek
// samping handler. Event duplikat dilewati.
type Registry struct {
	tx        usecase.Transactor
	processed ProcessedEventRepository
	log       *slog.Logger
	handlers  []registration
}

// NewRegistry membuat Registry.
func NewRegistry(tx usecase.Transactor, processed ProcessedEventRepository, log *slog.Logger) *Registry {
	return &Registry{tx: tx, processed: processed, log: log}
}

// Register mendaftarkan handler dengan nama consumer untuk jenis event
// tertentu (atau AnyEvent). Nama consumer harus unik dan stabil, karena
// dipakai sebagai kunci idempotency.
func (r *Registry) Register(consumer, eventType string, h Handler) {
	r.handlers = append(r.handlers, registration{consumer: consumer, eventType: eventType, handler: h})
}

// Topics mengembalikan topik Kafka yang perlu dikonsumsi untuk semua handler
// terdaftar.
func (r *Registry) Topics() []string {
	seen := map[string]bool{}
	var topics []string
	add := func(t string) {
		if !seen[t] {
			seen[t] = true
			topics = append(topics, t)
		}
	}
	for _, reg := range r.handlers {
		if reg.eventType == AnyEvent {
			for _, t := range []string{"users", "contacts", "addresses"} {
				add(t)
			}
			continue
		}
		add(model.TopicFor(reg.eventType))
	}
	return topics
}

// Handle menjalankan semua handler yang cocok dengan jenis event. Setiap
// handler berjalan di transaksinya sendiri bersama pencatatan idempotency,
// sehingga kegagalan satu handler tidak membatalkan handler lain yang sudah
// berhasil. Error dikembalikan agar pemanggil bisa mencoba ulang; handler
// yang sudah berhasil akan dilewati pada percobaan berikutnya.
func (r *Registry) Handle(ctx context.Context, e model.Event) error {
	for _, reg := range r.handlers {
		if reg.eventType != AnyEvent && reg.eventType != e.EventType {
			continue
		}
		err := r.tx.WithinTransaction(ctx, func(ctx context.Context) error {
			fresh, err := r.processed.TryMark(ctx, reg.consumer, e.EventID)
			if err != nil {
				return err
			}
			if !fresh {
				r.log.DebugContext(ctx, "duplicate event skipped", "consumer", reg.consumer, "event_id", e.EventID)
				return nil
			}
			return reg.handler(ctx, e)
		})
		if err != nil {
			return fmt.Errorf("consumer %s: %w", reg.consumer, err)
		}
	}
	return nil
}

// LocalDispatcher mengirim event outbox langsung ke handler di proses ini,
// tanpa Kafka. Dipakai saat Kafka dimatikan, sehingga fitur yang bergantung
// pada event (misalnya email selamat datang) tetap berjalan di mode minimal.
type LocalDispatcher struct {
	registry *Registry
}

// NewLocalDispatcher membuat LocalDispatcher.
func NewLocalDispatcher(registry *Registry) *LocalDispatcher {
	return &LocalDispatcher{registry: registry}
}

// Dispatch men-decode payload outbox dan menjalankan handler yang cocok.
func (d *LocalDispatcher) Dispatch(ctx context.Context, row entity.OutboxEvent) error {
	var e model.Event
	if err := json.Unmarshal(row.Payload, &e); err != nil {
		return fmt.Errorf("decode outbox payload: %w", err)
	}
	// Lanjutkan trace dari request yang memicu event ini.
	headers := map[string]string{}
	if len(row.Headers) > 0 {
		_ = json.Unmarshal(row.Headers, &headers)
	}
	ctx = observability.ExtractContext(ctx, headers)
	// Kegagalan handler lokal (misalnya server email down) diperlakukan
	// sebagai gangguan sementara: event tetap pending dan dicoba lagi, tidak
	// diparkir, sama seperti saat broker Kafka mati.
	if err := d.registry.Handle(ctx, e); err != nil {
		return outbox.Unavailable(err)
	}
	return nil
}
