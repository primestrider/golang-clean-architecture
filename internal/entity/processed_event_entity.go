package entity

// ProcessedEvent mencatat bahwa consumer tertentu sudah memproses event
// tertentu. Dipakai untuk idempotency.
type ProcessedEvent struct {
	ConsumerName string `gorm:"column:consumer_name;primaryKey"`
	EventID      string `gorm:"column:event_id;primaryKey"`
	ProcessedAt  int64  `gorm:"column:processed_at;autoCreateTime:milli"`
}

// TableName memberi tahu GORM nama tabel untuk entity ini.
func (p *ProcessedEvent) TableName() string {
	return "processed_events"
}
