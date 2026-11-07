// Package scheduler menjalankan pekerjaan terjadwal (cron) di worker,
// misalnya membersihkan data lama yang sudah tidak diperlukan.
package scheduler

import (
	"context"
	"errors"
	"time"
)

// Masa simpan data sebelum dihapus job pembersihan.
const (
	refreshTokenRetention    = 30 * 24 * time.Hour
	publishedOutboxRetention = 7 * 24 * time.Hour
	processedEventRetention  = 30 * 24 * time.Hour
)

// RefreshTokenCleaner menghapus refresh token lama.
type RefreshTokenCleaner interface {
	DeleteExpiredBefore(ctx context.Context, cutoff int64) (int64, error)
}

// OutboxCleaner menghapus event outbox yang sudah terkirim.
type OutboxCleaner interface {
	DeletePublishedBefore(ctx context.Context, cutoff int64) (int64, error)
}

// ProcessedEventCleaner menghapus catatan idempotency consumer yang lama.
type ProcessedEventCleaner interface {
	DeleteBefore(ctx context.Context, cutoff int64) (int64, error)
}

// Jobs berisi pekerjaan pembersihan. Now disuntikkan supaya batas waktu
// (cutoff) bisa diuji dengan waktu tetap.
type Jobs struct {
	RefreshTokens RefreshTokenCleaner
	Outbox        OutboxCleaner
	Processed     ProcessedEventCleaner
	Now           func() time.Time
}

// CleanupRefreshTokens menghapus refresh token yang kedaluwarsa atau
// dicabut lebih dari 30 hari lalu. Token itu tidak lagi berguna, tetapi
// disimpan sebentar untuk keperluan investigasi (misalnya deteksi reuse).
func (j Jobs) CleanupRefreshTokens(ctx context.Context) error {
	_, err := j.RefreshTokens.DeleteExpiredBefore(ctx, j.Now().Add(-refreshTokenRetention).UnixMilli())
	return err
}

// CleanupOutbox menghapus event outbox yang terkirim lebih dari 7 hari lalu
// dan catatan processed_events yang lebih dari 30 hari. Catatan idempotency
// disimpan lebih lama daripada outbox supaya event duplikat yang datang
// terlambat masih terdeteksi.
func (j Jobs) CleanupOutbox(ctx context.Context) error {
	_, errOutbox := j.Outbox.DeletePublishedBefore(ctx, j.Now().Add(-publishedOutboxRetention).UnixMilli())
	_, errProcessed := j.Processed.DeleteBefore(ctx, j.Now().Add(-processedEventRetention).UnixMilli())
	return errors.Join(errOutbox, errProcessed)
}
