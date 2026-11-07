//go:build integration

package test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/primestrider/golang-clean-architecture/internal/delivery/messaging"
	"github.com/primestrider/golang-clean-architecture/internal/entity"
	"github.com/primestrider/golang-clean-architecture/internal/model"
	"github.com/primestrider/golang-clean-architecture/internal/repository"
	"github.com/primestrider/golang-clean-architecture/test/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newRegistry(app *testsupport.TestApp) *messaging.Registry {
	return messaging.NewRegistry(repository.NewTransactor(app.DB), repository.NewProcessedEventRepository(app.DB), slog.New(slog.DiscardHandler))
}

// TestRegistry_DuplicateEventProcessedOnce: event yang terkirim ulang
// (misalnya setelah worker restart) hanya diproses sekali per consumer.
func TestRegistry_DuplicateEventProcessedOnce(t *testing.T) {
	app := testsupport.NewWebApp(t)
	registry := newRegistry(app)
	calls := map[string]int{}
	registry.Register("welcome-email", "user.registered", func(_ context.Context, e model.Event) error {
		calls["welcome-email"]++
		return nil
	})
	registry.Register("crm-sync", "user.registered", func(_ context.Context, e model.Event) error {
		calls["crm-sync"]++
		return nil
	})
	e, err := model.NewEvent("user.registered", uuid.NewString(), "", map[string]string{})
	require.NoError(t, err)

	require.NoError(t, registry.Handle(context.Background(), e))
	require.NoError(t, registry.Handle(context.Background(), e))

	assert.Equal(t, map[string]int{"welcome-email": 1, "crm-sync": 1}, calls)
}

func TestRegistry_FailedHandlerIsNotMarkedProcessed(t *testing.T) {
	app := testsupport.NewWebApp(t)
	registry := newRegistry(app)
	fail := true
	calls := 0
	registry.Register("flaky", "contact.created", func(context.Context, model.Event) error {
		calls++
		if fail {
			return errors.New("temporary failure")
		}
		return nil
	})
	e, err := model.NewEvent("contact.created", uuid.NewString(), "", map[string]string{})
	require.NoError(t, err)

	require.Error(t, registry.Handle(context.Background(), e))
	fail = false
	require.NoError(t, registry.Handle(context.Background(), e))

	assert.Equal(t, 2, calls, "a failed attempt must be retried, not skipped")
}

func TestLocalDispatcher_RunsHandlersFromOutboxRow(t *testing.T) {
	app := testsupport.NewWebApp(t)
	registry := newRegistry(app)
	var got model.Event
	registry.Register("test", "contact.created", func(_ context.Context, e model.Event) error {
		got = e
		return nil
	})
	e, err := model.NewEvent("contact.created", uuid.NewString(), "u1", map[string]string{"first_name": "Budi"})
	require.NoError(t, err)
	payload, err := json.Marshal(e)
	require.NoError(t, err)

	err = messaging.NewLocalDispatcher(registry).Dispatch(context.Background(), entity.OutboxEvent{ID: e.EventID, Payload: payload})

	require.NoError(t, err)
	assert.Equal(t, e.EventID, got.EventID)
	assert.JSONEq(t, `{"first_name":"Budi"}`, string(got.Data))
}

// discard mengembalikan logger yang membuang semua output.
func discard() *slog.Logger { return slog.New(slog.DiscardHandler) }
