package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/IBM/sarama/mocks"
	"github.com/primestrider/golang-clean-architecture/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeHandler menghitung pemanggilan dan selalu mengembalikan err.
type fakeHandler struct {
	calls int
	err   error
}

func (h *fakeHandler) Handle(context.Context, model.Event) error {
	h.calls++
	return h.err
}

func message(t *testing.T) *sarama.ConsumerMessage {
	e, err := model.NewEvent("contact.created", "c1", "u1", map[string]string{"id": "c1"})
	require.NoError(t, err)
	raw, err := json.Marshal(e)
	require.NoError(t, err)
	return &sarama.ConsumerMessage{Topic: "contacts", Key: []byte("c1"), Value: raw}
}

func headerMap(msg *sarama.ProducerMessage) map[string]string {
	out := map[string]string{}
	for _, h := range msg.Headers {
		out[string(h.Key)] = string(h.Value)
	}
	return out
}

func TestProcessor_SuccessDoesNotUseDLQ(t *testing.T) {
	handler := &fakeHandler{}
	dlq := mocks.NewSyncProducer(t, nil)
	p := NewProcessor(handler, dlq, 3, time.Millisecond, slog.New(slog.DiscardHandler))

	require.NoError(t, p.Process(context.Background(), message(t)))

	assert.Equal(t, 1, handler.calls)
}

func TestProcessor_RetriesThenDLQ(t *testing.T) {
	handler := &fakeHandler{err: errors.New("smtp down")}
	dlq := mocks.NewSyncProducer(t, nil)
	var sent *sarama.ProducerMessage
	dlq.ExpectSendMessageWithMessageCheckerFunctionAndSucceed(func(msg *sarama.ProducerMessage) error {
		sent = msg
		return nil
	})
	p := NewProcessor(handler, dlq, 3, time.Millisecond, slog.New(slog.DiscardHandler))

	err := p.Process(context.Background(), message(t))

	require.NoError(t, err, "after DLQ the offset may be committed")
	assert.Equal(t, 4, handler.calls, "1 attempt + 3 retries")
	require.NotNil(t, sent)
	assert.Equal(t, "contacts.dlq", sent.Topic)
	h := headerMap(sent)
	assert.Equal(t, "smtp down", h["x-error"])
	assert.Equal(t, "contacts", h["x-original-topic"])
	assert.Equal(t, "4", h["x-attempts"])
}

func TestProcessor_PoisonMessageGoesStraightToDLQ(t *testing.T) {
	handler := &fakeHandler{}
	dlq := mocks.NewSyncProducer(t, nil)
	dlq.ExpectSendMessageWithMessageCheckerFunctionAndSucceed(func(msg *sarama.ProducerMessage) error {
		if msg.Topic != "contacts.dlq" {
			return errors.New("wrong topic " + msg.Topic)
		}
		return nil
	})
	p := NewProcessor(handler, dlq, 3, time.Millisecond, slog.New(slog.DiscardHandler))

	err := p.Process(context.Background(), &sarama.ConsumerMessage{Topic: "contacts", Value: []byte("{not json")})

	require.NoError(t, err)
	assert.Zero(t, handler.calls)
}

func TestProcessor_DLQFailureReturnsError(t *testing.T) {
	handler := &fakeHandler{err: errors.New("boom")}
	dlq := mocks.NewSyncProducer(t, nil)
	dlq.ExpectSendMessageAndFail(errors.New("kafka down"))
	p := NewProcessor(handler, dlq, 0, time.Millisecond, slog.New(slog.DiscardHandler))

	err := p.Process(context.Background(), message(t))

	assert.Error(t, err, "offset must not be committed when the DLQ write fails")
}
