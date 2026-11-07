package entity

import "gorm.io/plugin/soft_delete"

// Address adalah alamat milik sebuah Contact.
type Address struct {
	ID         string `gorm:"column:id;primaryKey"`
	ContactID  string `gorm:"column:contact_id"`
	Street     string `gorm:"column:street"`
	City       string `gorm:"column:city"`
	Province   string `gorm:"column:province"`
	PostalCode string `gorm:"column:postal_code"`
	Country    string `gorm:"column:country"`
	CreatedAt  int64  `gorm:"column:created_at;autoCreateTime:milli"`
	UpdatedAt  int64  `gorm:"column:updated_at;autoCreateTime:milli;autoUpdateTime:milli"`
	// Version dipakai untuk optimistic locking.
	Version int `gorm:"column:version;default:1"`
	// DeletedAt bernilai 0 untuk data aktif (soft delete).
	DeletedAt soft_delete.DeletedAt `gorm:"column:deleted_at;softDelete:milli"`
}

// TableName memberi tahu GORM nama tabel untuk entity ini.
func (a *Address) TableName() string {
	return "addresses"
}

// GetID mengembalikan primary key.
func (a *Address) GetID() string { return a.ID }

// GetVersion mengembalikan versi data untuk optimistic locking.
func (a *Address) GetVersion() int { return a.Version }

// SetVersion mengubah versi data.
func (a *Address) SetVersion(v int) { a.Version = v }
