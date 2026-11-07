//go:build integration

package test

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
	"github.com/primestrider/golang-clean-architecture/internal/entity"
	"github.com/primestrider/golang-clean-architecture/internal/outbox"
	"github.com/primestrider/golang-clean-architecture/internal/repository"
	"github.com/primestrider/golang-clean-architecture/test/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func insertOutbox(t *testing.T, app *testsupport.TestApp, aggregate string, n int) []string {
	t.Helper()
	ids := make([]string, n)
	for i := range n {
		ids[i] = uuid.NewString()
		require.NoError(t, app.DB.Create(&entity.OutboxEvent{
			ID: ids[i], Topic: "contacts", MsgKey: aggregate, AggregateID: aggregate,
			EventType: "contact.updated", Payload: []byte(`{}`), Status: entity.OutboxPending,
			CreatedAt: time.Now().UnixMilli() + int64(i),
		}).Error)
	}
	return ids
}

func statusOf(t *testing.T, app *testsupport.TestApp, id string) entity.OutboxEvent {
	t.Helper()
	var e entity.OutboxEvent
	require.NoError(t, app.DB.First(&e, "id = ?", id).Error)
	return e
}

func TestMySQLLocker_OnlyOneHolder(t *testing.T) {
	app := testsupport.NewWebApp(t)
	ctx := context.Background()
	first := outbox.NewMySQLLocker(app.DB, "test-relay")
	second := outbox.NewMySQLLocker(app.DB, "test-relay")

	ok, err := first.TryLock(ctx)
	require.NoError(t, err)
	assert.True(t, ok)
	ok, err = first.TryLock(ctx)
	require.NoError(t, err)
	assert.True(t, ok, "holder keeps the lock on later checks")
	ok, err = second.TryLock(ctx)
	require.NoError(t, err)
	assert.False(t, ok, "only one locker may hold the lock")

	require.NoError(t, first.Unlock(ctx))
	ok, err = second.TryLock(ctx)
	require.NoError(t, err)
	assert.True(t, ok, "lock is available after release")
	require.NoError(t, second.Unlock(ctx))
}

func TestOutboxRepository_FailedEventBlocksItsAggregate(t *testing.T) {
	app := testsupport.NewWebApp(t)
	repo := repository.NewOutboxRepository(app.DB)
	ctx := context.Background()
	blocked := insertOutbox(t, app, uuid.NewString(), 2)
	other := insertOutbox(t, app, uuid.NewString(), 1)

	require.NoError(t, repo.MarkFailedAttempt(ctx, blocked[0], "boom", 2))
	assert.Equal(t, entity.OutboxPending, statusOf(t, app, blocked[0]).Status, "below max attempts stays pending")
	require.NoError(t, repo.MarkFailedAttempt(ctx, blocked[0], "boom", 2))
	parked := statusOf(t, app, blocked[0])
	assert.Equal(t, entity.OutboxFailed, parked.Status)
	assert.Equal(t, 2, parked.Attempts)

	pending, err := repo.FetchPending(ctx, 100)
	require.NoError(t, err)
	require.Len(t, pending, 1, "pending events of a parked aggregate are held back")
	assert.Equal(t, other[0], pending[0].ID)
}

// flakyDispatcher mensimulasikan broker yang mati lalu hidup lagi.
type flakyDispatcher struct {
	down atomic.Bool
	sent atomic.Int32
}

func (d *flakyDispatcher) Dispatch(context.Context, entity.OutboxEvent) error {
	if d.down.Load() {
		return outbox.Unavailable(errors.New("kafka: client has run out of available brokers"))
	}
	d.sent.Add(1)
	return nil
}

// TestRelay_BrokerDownKeepsPendingThenPublishes: selama broker mati, event
// tetap pending (tidak hilang); setelah broker hidup, event terkirim.
func TestRelay_BrokerDownKeepsPendingThenPublishes(t *testing.T) {
	app := testsupport.NewWebApp(t)
	ids := insertOutbox(t, app, uuid.NewString(), 3)
	dispatcher := &flakyDispatcher{}
	dispatcher.down.Store(true)
	relay := outbox.NewRelay(repository.NewOutboxRepository(app.DB), dispatcher,
		outbox.NewMySQLLocker(app.DB, "test-relay-outage"),
		// MaxAttempts default (10): gangguan broker tidak boleh membuat event diparkir.
		outbox.Options{PollInterval: 20 * time.Millisecond, BatchSize: 100, MaxAttempts: 10},
		slog.New(slog.DiscardHandler))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = relay.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })

	// Tunggu jauh lebih lama dari 10 putaran (MaxAttempts) selama broker mati.
	time.Sleep(600 * time.Millisecond)
	for _, id := range ids {
		row := statusOf(t, app, id)
		assert.Equal(t, entity.OutboxPending, row.Status)
		assert.Zero(t, row.Attempts, "broker outages must not count as failed attempts")
	}

	dispatcher.down.Store(false)
	require.Eventually(t, func() bool {
		for _, id := range ids {
			if statusOf(t, app, id).Status != entity.OutboxPublished {
				return false
			}
		}
		return true
	}, 5*time.Second, 20*time.Millisecond)
	assert.EqualValues(t, 3, dispatcher.sent.Load())
}

func TestKafkaDispatcher_PublishesToBroker(t *testing.T) {
	brokers := testsupport.StartKafka(t)
	sc := sarama.NewConfig()
	sc.Producer.Return.Successes = true
	producer, err := sarama.NewSyncProducer(brokers, sc)
	require.NoError(t, err)
	t.Cleanup(func() { _ = producer.Close() })
	aggregate := uuid.NewString()

	err = outbox.NewKafkaDispatcher(producer).Dispatch(context.Background(), entity.OutboxEvent{
		ID: uuid.NewString(), Topic: "contacts", MsgKey: aggregate, AggregateID: aggregate,
		EventType: "contact.created", Payload: []byte(`{"event_type":"contact.created"}`),
		Headers: []byte(`{"traceparent":"00-0123456789abcdef0123456789abcdef-0123456789abcdef-01"}`),
	})
	require.NoError(t, err)

	consumer, err := sarama.NewConsumer(brokers, sarama.NewConfig())
	require.NoError(t, err)
	t.Cleanup(func() { _ = consumer.Close() })
	pc, err := consumer.ConsumePartition("contacts", 0, sarama.OffsetOldest)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pc.Close() })

	select {
	case msg := <-pc.Messages():
		assert.Equal(t, aggregate, string(msg.Key))
		headers := map[string]string{}
		for _, h := range msg.Headers {
			headers[string(h.Key)] = string(h.Value)
		}
		assert.Equal(t, "contact.created", headers["event_type"])
		assert.NotEmpty(t, headers["traceparent"])
	case <-time.After(30 * time.Second):
		t.Fatal("message not received from kafka")
	}
}
