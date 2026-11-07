package outbox

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/primestrider/golang-clean-architecture/internal/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRepo adalah Repository di memori untuk unit test.
type fakeRepo struct {
	mu        sync.Mutex
	pending   []entity.OutboxEvent
	published []string
	failed    map[string]int
	maxSeen   int
}

func (f *fakeRepo) FetchPending(_ context.Context, limit int) ([]entity.OutboxEvent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.pending) > limit {
		return f.pending[:limit], nil
	}
	return f.pending, nil
}

func (f *fakeRepo) MarkPublished(_ context.Context, id string, _ int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.published = append(f.published, id)
	return nil
}

func (f *fakeRepo) MarkFailedAttempt(_ context.Context, id, _ string, maxAttempts int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failed == nil {
		f.failed = map[string]int{}
	}
	f.failed[id]++
	f.maxSeen = maxAttempts
	return nil
}

// fakeDispatcher mencatat event yang dikirim; event di failIDs selalu gagal.
type fakeDispatcher struct {
	sent    []string
	failIDs map[string]bool
}

func (d *fakeDispatcher) Dispatch(_ context.Context, e entity.OutboxEvent) error {
	if d.failIDs[e.ID] {
		return errors.New("broker unavailable")
	}
	d.sent = append(d.sent, e.ID)
	return nil
}

// fakeLocker mengembalikan hasil TryLock yang sudah ditentukan.
type fakeLocker struct{ acquired bool }

func (l *fakeLocker) TryLock(context.Context) (bool, error) { return l.acquired, nil }
func (l *fakeLocker) Unlock(context.Context) error          { return nil }

func testCfg() Options {
	return Options{PollInterval: 10 * time.Millisecond, BatchSize: 100, MaxAttempts: 10}
}

func event(id, aggregate string) entity.OutboxEvent {
	return entity.OutboxEvent{ID: id, AggregateID: aggregate}
}

func TestProcessBatch_PreservesOrderPerAggregate(t *testing.T) {
	repo := &fakeRepo{pending: []entity.OutboxEvent{event("A1", "a"), event("B1", "b"), event("A2", "a"), event("B2", "b")}}
	dispatcher := &fakeDispatcher{failIDs: map[string]bool{"A1": true}}
	relay := NewRelay(repo, dispatcher, &fakeLocker{acquired: true}, testCfg(), slog.New(slog.DiscardHandler))

	published, err := relay.ProcessBatch(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 2, published)
	assert.Equal(t, []string{"B1", "B2"}, dispatcher.sent, "A2 must wait until A1 succeeds")
	assert.Equal(t, map[string]int{"A1": 1}, repo.failed)
	assert.Equal(t, []string{"B1", "B2"}, repo.published)
}

func TestProcessBatch_PassesMaxAttemptsToRepository(t *testing.T) {
	repo := &fakeRepo{pending: []entity.OutboxEvent{event("A1", "a")}}
	cfg := testCfg()
	cfg.MaxAttempts = 3
	relay := NewRelay(repo, &fakeDispatcher{failIDs: map[string]bool{"A1": true}}, &fakeLocker{acquired: true}, cfg, slog.New(slog.DiscardHandler))

	_, err := relay.ProcessBatch(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 3, repo.maxSeen)
}

func TestRun_SkipsWhenLockNotAcquired(t *testing.T) {
	repo := &fakeRepo{pending: []entity.OutboxEvent{event("A1", "a")}}
	dispatcher := &fakeDispatcher{}
	relay := NewRelay(repo, dispatcher, &fakeLocker{acquired: false}, testCfg(), slog.New(slog.DiscardHandler))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()

	err := relay.Run(ctx)

	require.NoError(t, err)
	assert.Empty(t, dispatcher.sent)
}

func TestRun_PublishesWhenLeader(t *testing.T) {
	repo := &fakeRepo{pending: []entity.OutboxEvent{event("A1", "a")}}
	dispatcher := &fakeDispatcher{}
	relay := NewRelay(repo, dispatcher, &fakeLocker{acquired: true}, testCfg(), slog.New(slog.DiscardHandler))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()

	require.NoError(t, relay.Run(ctx))

	assert.Contains(t, dispatcher.sent, "A1")
}
