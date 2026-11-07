package entity

// Status baris outbox.
const (
	// OutboxPending menunggu dikirim relay.
	OutboxPending = "pending"
	// OutboxPublished sudah berhasil dikirim.
	OutboxPublished = "published"
	// OutboxFailed gagal terus sampai batas percobaan dan diparkir. Event
	// berikutnya untuk aggregate yang sama ikut tertahan supaya urutan tetap
	// terjaga, sampai operator memperbaikinya.
	OutboxFailed = "failed"
)

// OutboxEvent adalah satu event yang menunggu (atau sudah) dikirim ke
// message broker. Ditulis di transaksi yang sama dengan perubahan data.
type OutboxEvent struct {
	ID          string `gorm:"column:id;primaryKey"`
	Topic       string `gorm:"column:topic"`
	MsgKey      string `gorm:"column:msg_key"`
	AggregateID string `gorm:"column:aggregate_id"`
	EventType   string `gorm:"column:event_type"`
	// Payload berisi model.Event lengkap dalam JSON.
	Payload []byte `gorm:"column:payload;type:json"`
	// Headers berisi header pesan (misalnya konteks tracing) dalam JSON.
	Headers     []byte  `gorm:"column:headers;type:json"`
	Status      string  `gorm:"column:status"`
	Attempts    int     `gorm:"column:attempts"`
	LastError   *string `gorm:"column:last_error"`
	CreatedAt   int64   `gorm:"column:created_at;autoCreateTime:milli"`
	PublishedAt *int64  `gorm:"column:published_at"`
}

// TableName memberi tahu GORM nama tabel untuk entity ini.
func (o *OutboxEvent) TableName() string {
	return "outbox_events"
}
