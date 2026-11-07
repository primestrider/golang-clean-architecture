//go:build integration

package test

import (
	"context"
	"testing"
	"time"

	"github.com/primestrider/golang-clean-architecture/internal/scheduler"
	"github.com/primestrider/golang-clean-architecture/test/testsupport"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedisLocker_OnlyOneWorkerRunsAJob(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: testsupport.StartRedis(t)})
	first := scheduler.NewRedisLocker(client, time.Minute)
	second := scheduler.NewRedisLocker(client, time.Minute)
	ctx := context.Background()

	lock, err := first.Lock(ctx, "cleanup-outbox")
	require.NoError(t, err)
	_, err = second.Lock(ctx, "cleanup-outbox")
	assert.Error(t, err, "second worker must skip while the lock is held")

	require.NoError(t, lock.Unlock(ctx))
	_, err = second.Lock(ctx, "cleanup-outbox")
	assert.NoError(t, err)
}

func TestScheduler_RegistersCleanupJobs(t *testing.T) {
	s, err := scheduler.New(scheduler.Options{TokenCleanupCron: "0 3 * * *", OutboxCleanupCron: "30 3 * * *"},
		nil, discard(), scheduler.Jobs{Now: time.Now})
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Shutdown() })

	var names []string
	for _, j := range s.Jobs() {
		names = append(names, j.Name())
	}
	assert.ElementsMatch(t, []string{"cleanup-refresh-tokens", "cleanup-outbox"}, names)
}

func TestScheduler_RejectsInvalidCron(t *testing.T) {
	_, err := scheduler.New(scheduler.Options{TokenCleanupCron: "not a cron", OutboxCleanupCron: "30 3 * * *"},
		nil, discard(), scheduler.Jobs{Now: time.Now})

	assert.Error(t, err)
}
