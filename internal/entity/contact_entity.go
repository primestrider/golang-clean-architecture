package entity

import "gorm.io/plugin/soft_delete"

// Contact adalah kontak milik seorang user. Kepemilikan (UserID) selalu
// dicek di setiap query supaya user tidak bisa membaca kontak user lain.
type Contact struct {
	ID        string `gorm:"column:id;primaryKey"`
	FirstName string `gorm:"column:first_name"`
	LastName  string `gorm:"column:last_name"`
	Email     string `gorm:"column:email"`
	Phone     string `gorm:"column:phone"`
	UserID    string `gorm:"column:user_id"`
	CreatedAt int64  `gorm:"column:created_at;autoCreateTime:milli"`
	UpdatedAt int64  `gorm:"column:updated_at;autoCreateTime:milli;autoUpdateTime:milli"`
	// Version dipakai untuk optimistic locking.
	Version int `gorm:"column:version;default:1"`
	// DeletedAt bernilai 0 untuk data aktif (soft delete).
	DeletedAt soft_delete.DeletedAt `gorm:"column:deleted_at;softDelete:milli"`
}

// TableName memberi tahu GORM nama tabel untuk entity ini.
func (c *Contact) TableName() string {
	return "contacts"
}

// GetID mengembalikan primary key.
func (c *Contact) GetID() string { return c.ID }

// GetVersion mengembalikan versi data untuk optimistic locking.
func (c *Contact) GetVersion() int { return c.Version }

// SetVersion mengubah versi data.
func (c *Contact) SetVersion(v int) { c.Version = v }
