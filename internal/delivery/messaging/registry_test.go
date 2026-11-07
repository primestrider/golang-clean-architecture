package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"testing"

	"github.com/primestrider/golang-clean-architecture/internal/entity"
	"github.com/primestrider/golang-clean-architecture/internal/model"
	"github.com/primestrider/golang-clean-architecture/internal/outbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type passTx struct{}

func (passTx) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

type alwaysFresh struct{}

func (alwaysFresh) TryMark(context.Context, string, string) (bool, error) { return true, nil }

func dispatchWithHandlerError(t *testing.T, handlerErr error) error {
	t.Helper()
	registry := NewRegistry(passTx{}, alwaysFresh{}, slog.New(slog.DiscardHandler))
	registry.Register("test", "user.registered", func(context.Context, model.Event) error { return handlerErr })
	e, err := model.NewEvent("user.registered", "u1", "", map[string]string{})
	require.NoError(t, err)
	payload, err := json.Marshal(e)
	require.NoError(t, err)
	return NewLocalDispatcher(registry).Dispatch(context.Background(), entity.OutboxEvent{Payload: payload})
}

func TestLocalDispatcher_NetworkErrorIsUnavailable(t *testing.T) {
	err := dispatchWithHandlerError(t, &net.OpError{Op: "dial", Err: errors.New("connection refused")})

	assert.ErrorIs(t, err, outbox.ErrUnavailable)
}

func TestLocalDispatcher_PermanentErrorCountsAsAttempt(t *testing.T) {
	err := dispatchWithHandlerError(t, errors.New("550 mailbox does not exist"))

	require.Error(t, err)
	assert.NotErrorIs(t, err, outbox.ErrUnavailable, "permanent failures must be counted so the event is eventually parked")
}

// tempErr meniru error SMTP 4xx dari go-mail (punya method IsTemp).
type tempErr struct{}

func (tempErr) Error() string { return "421 service not available" }
func (tempErr) IsTemp() bool  { return true }

func TestLocalDispatcher_TemporaryErrorIsUnavailable(t *testing.T) {
	assert.ErrorIs(t, dispatchWithHandlerError(t, tempErr{}), outbox.ErrUnavailable)
}
