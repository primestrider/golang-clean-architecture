package entity

// AuditLog adalah satu catatan jejak audit.
type AuditLog struct {
	ID string `gorm:"column:id;primaryKey"`
	// ActorID adalah user yang melakukan aksi; nil untuk aksi sistem atau
	// aksi tanpa login (misalnya registrasi).
	ActorID    *string `gorm:"column:actor_id"`
	Action     string  `gorm:"column:action"`
	EntityType string  `gorm:"column:entity_type"`
	EntityID   string  `gorm:"column:entity_id"`
	RequestID  *string `gorm:"column:request_id"`
	// Changes berisi JSON {"field": [sebelum, sesudah]}.
	Changes   []byte `gorm:"column:changes;type:json"`
	CreatedAt int64  `gorm:"column:created_at;autoCreateTime:milli"`
}

// TableName memberi tahu GORM nama tabel untuk entity ini.
func (a *AuditLog) TableName() string {
	return "audit_logs"
}
