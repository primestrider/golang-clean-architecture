package usecase

import (
	"context"
	"encoding/json"

	"github.com/primestrider/golang-clean-architecture/internal/apperror"
	"github.com/primestrider/golang-clean-architecture/internal/entity"
	"github.com/primestrider/golang-clean-architecture/internal/model"
	"github.com/primestrider/golang-clean-architecture/internal/observability"
)

// OutboxPublisher adalah EventPublisher yang menulis event ke tabel outbox,
// bukan langsung ke Kafka (pola transactional outbox).
//
// Mengirim ke Kafka langsung dari use case punya dua masalah: bila Kafka
// mati setelah data tersimpan, event hilang; dan bila transaksi gagal
// setelah event terkirim, consumer menerima event untuk data yang tidak
// pernah ada. Dengan outbox, event disimpan di transaksi yang sama dengan
// data, lalu relay di worker yang mengirimkannya.
type OutboxPublisher struct {
	outbox OutboxRepository
}

// NewOutboxPublisher membuat OutboxPublisher.
func NewOutboxPublisher(outbox OutboxRepository) *OutboxPublisher {
	return &OutboxPublisher{outbox: outbox}
}

// Publish menyimpan event ke outbox memakai transaksi yang ada di ctx.
// Header berisi konteks tracing supaya trace bisa diteruskan ke consumer.
func (p *OutboxPublisher) Publish(ctx context.Context, events ...model.Event) error {
	headers, err := json.Marshal(observability.InjectHeaders(ctx))
	if err != nil {
		return apperror.Internal(err)
	}
	rows := make([]entity.OutboxEvent, 0, len(events))
	for _, e := range events {
		payload, err := json.Marshal(e)
		if err != nil {
			return apperror.Internal(err)
		}
		rows = append(rows, entity.OutboxEvent{
			ID:          e.EventID,
			Topic:       model.TopicFor(e.EventType),
			MsgKey:      e.AggregateID,
			AggregateID: e.AggregateID,
			EventType:   e.EventType,
			Payload:     payload,
			Headers:     headers,
			Status:      entity.OutboxPending,
		})
	}
	return p.outbox.Insert(ctx, rows...)
}

// newEvent membuat event dengan pelaku diambil dari context (user yang login).
func newEvent(ctx context.Context, eventType, aggregateID string, data any) (model.Event, error) {
	e, err := model.NewEvent(eventType, aggregateID, observability.UserID(ctx), data)
	if err != nil {
		return model.Event{}, apperror.Internal(err)
	}
	return e, nil
}

// publish membuat satu event dan menerbitkannya lewat publisher.
func publish(ctx context.Context, events EventPublisher, eventType, aggregateID string, data any) error {
	e, err := newEvent(ctx, eventType, aggregateID, data)
	if err != nil {
		return err
	}
	return events.Publish(ctx, e)
}
