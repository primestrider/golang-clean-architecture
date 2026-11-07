package repository

import (
	"context"

	"github.com/primestrider/golang-clean-architecture/internal/entity"
	"gorm.io/gorm"
)

// OutboxRepository mengakses tabel outbox_events.
type OutboxRepository struct {
	Repository[entity.OutboxEvent]
}

// NewOutboxRepository membuat OutboxRepository.
func NewOutboxRepository(db *gorm.DB) *OutboxRepository {
	return &OutboxRepository{Repository: Repository[entity.OutboxEvent]{DB: db, Name: "outbox event"}}
}

// Insert menyimpan satu atau lebih event baru. Dipanggil di dalam transaksi
// bisnis lewat ctx.
func (r *OutboxRepository) Insert(ctx context.Context, events ...entity.OutboxEvent) error {
	if len(events) == 0 {
		return nil
	}
	return r.db(ctx).Create(&events).Error
}

// FetchPending mengambil event pending tertua (urut created_at, lalu id),
// kecuali event milik aggregate yang punya event berstatus failed. Event
// failed menahan event berikutnya untuk aggregate yang sama supaya urutan
// per aggregate tidak pernah terbalik.
func (r *OutboxRepository) FetchPending(ctx context.Context, limit int) ([]entity.OutboxEvent, error) {
	var events []entity.OutboxEvent
	err := r.db(ctx).
		Where("status = ?", entity.OutboxPending).
		Where("aggregate_id NOT IN (?)", r.db(ctx).Model(&entity.OutboxEvent{}).
			Select("aggregate_id").Where("status = ?", entity.OutboxFailed)).
		Order("created_at, id").
		Limit(limit).
		Find(&events).Error
	return events, err
}

// MarkPublished menandai event sudah terkirim pada waktu at (epoch ms).
func (r *OutboxRepository) MarkPublished(ctx context.Context, id string, at int64) error {
	return r.db(ctx).Model(&entity.OutboxEvent{}).Where("id = ?", id).
		Updates(map[string]any{"status": entity.OutboxPublished, "published_at": at}).Error
}

// MarkFailedAttempt mencatat satu percobaan kirim yang gagal. Bila jumlah
// percobaan mencapai maxAttempts, status menjadi failed (diparkir).
func (r *OutboxRepository) MarkFailedAttempt(ctx context.Context, id, errMsg string, maxAttempts int) error {
	return r.db(ctx).Model(&entity.OutboxEvent{}).Where("id = ?", id).
		Updates(map[string]any{
			"attempts":   gorm.Expr("attempts + 1"),
			"last_error": errMsg,
			"status":     gorm.Expr("CASE WHEN attempts + 1 >= ? THEN ? ELSE status END", maxAttempts, entity.OutboxFailed),
		}).Error
}

// CountByStatus menghitung jumlah event per status (untuk metrik).
func (r *OutboxRepository) CountByStatus(ctx context.Context) (map[string]int64, error) {
	var rows []struct {
		Status string
		Total  int64
	}
	if err := r.db(ctx).Model(&entity.OutboxEvent{}).Select("status, COUNT(*) AS total").Group("status").Scan(&rows).Error; err != nil {
		return nil, err
	}
	counts := map[string]int64{}
	for _, row := range rows {
		counts[row.Status] = row.Total
	}
	return counts, nil
}

// DeletePublishedBefore menghapus event yang sudah terkirim sebelum cutoff
// (epoch ms) dan mengembalikan jumlah yang dihapus.
func (r *OutboxRepository) DeletePublishedBefore(ctx context.Context, cutoff int64) (int64, error) {
	res := r.db(ctx).Where("status = ? AND published_at < ?", entity.OutboxPublished, cutoff).Delete(&entity.OutboxEvent{})
	return res.RowsAffected, res.Error
}
