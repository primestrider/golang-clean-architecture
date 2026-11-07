package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cutoffRecorder mencatat cutoff yang diterima setiap pembersih.
type cutoffRecorder struct{ cutoff int64 }

func (r *cutoffRecorder) DeleteExpiredBefore(_ context.Context, cutoff int64) (int64, error) {
	r.cutoff = cutoff
	return 3, nil
}

func (r *cutoffRecorder) DeletePublishedBefore(_ context.Context, cutoff int64) (int64, error) {
	r.cutoff = cutoff
	return 2, nil
}

func (r *cutoffRecorder) DeleteBefore(_ context.Context, cutoff int64) (int64, error) {
	r.cutoff = cutoff
	return 1, nil
}

var now = time.Date(2026, 3, 31, 3, 0, 0, 0, time.UTC)

func TestCleanupRefreshTokens_Uses30DayCutoff(t *testing.T) {
	tokens := &cutoffRecorder{}
	jobs := Jobs{RefreshTokens: tokens, Now: func() time.Time { return now }}

	require.NoError(t, jobs.CleanupRefreshTokens(context.Background()))

	assert.Equal(t, now.AddDate(0, 0, -30).UnixMilli(), tokens.cutoff)
}

func TestCleanupOutbox_Uses7And30DayCutoffs(t *testing.T) {
	outbox, processed := &cutoffRecorder{}, &cutoffRecorder{}
	jobs := Jobs{Outbox: outbox, Processed: processed, Now: func() time.Time { return now }}

	require.NoError(t, jobs.CleanupOutbox(context.Background()))

	assert.Equal(t, now.AddDate(0, 0, -7).UnixMilli(), outbox.cutoff)
	assert.Equal(t, now.AddDate(0, 0, -30).UnixMilli(), processed.cutoff)
}
