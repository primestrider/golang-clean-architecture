package usecase

import (
	"context"
	"log/slog"

	"github.com/primestrider/golang-clean-architecture/internal/model"
)

// NoopEventPublisher adalah EventPublisher sementara yang hanya mencatat
// event ke log. Implementasi ini akan digantikan oleh outbox publisher.
type NoopEventPublisher struct {
	log *slog.Logger
}

// NewNoopEventPublisher membuat NoopEventPublisher.
func NewNoopEventPublisher(log *slog.Logger) *NoopEventPublisher {
	return &NoopEventPublisher{log: log}
}

// Publish mencatat id setiap event di level debug.
func (p *NoopEventPublisher) Publish(ctx context.Context, events ...model.Event) error {
	for _, e := range events {
		p.log.DebugContext(ctx, "event not published (noop publisher)", "event_id", e.GetId())
	}
	return nil
}
