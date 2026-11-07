package repository

import (
	"context"

	"github.com/primestrider/golang-clean-architecture/internal/entity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ProcessedEventRepository mengakses tabel processed_events.
type ProcessedEventRepository struct {
	Repository[entity.ProcessedEvent]
}

// NewProcessedEventRepository membuat ProcessedEventRepository.
func NewProcessedEventRepository(db *gorm.DB) *ProcessedEventRepository {
	return &ProcessedEventRepository{Repository: Repository[entity.ProcessedEvent]{DB: db, Name: "processed event"}}
}

// TryMark mencatat (consumer, eventID) dengan INSERT IGNORE. Bila baris
// sudah ada (primary key bentrok), tidak ada baris yang masuk dan hasilnya
// false: event ini duplikat. Mengandalkan primary key membuat pengecekan
// aman walau dua worker memproses event yang sama bersamaan.
func (r *ProcessedEventRepository) TryMark(ctx context.Context, consumer, eventID string) (bool, error) {
	res := r.db(ctx).Clauses(clause.Insert{Modifier: "IGNORE"}).
		Create(&entity.ProcessedEvent{ConsumerName: consumer, EventID: eventID})
	return res.RowsAffected == 1, res.Error
}

// DeleteBefore menghapus catatan yang lebih tua dari cutoff (epoch ms).
func (r *ProcessedEventRepository) DeleteBefore(ctx context.Context, cutoff int64) (int64, error) {
	res := r.db(ctx).Where("processed_at < ?", cutoff).Delete(&entity.ProcessedEvent{})
	return res.RowsAffected, res.Error
}
