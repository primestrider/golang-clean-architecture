package entity

import "gorm.io/plugin/soft_delete"

// Role yang dikenal aplikasi. Role ikut tersimpan di access token, sehingga
// middleware otorisasi tidak perlu ke database di setiap request.
const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

// User adalah pemilik akun. Setiap user memiliki banyak Contact.
type User struct {
	ID    string `gorm:"column:id;primaryKey"`
	Email string `gorm:"column:email"`
	Name  string `gorm:"column:name"`
	// Password berisi hash bcrypt, tidak pernah password asli.
	Password string `gorm:"column:password"`
	Role     string `gorm:"column:role"`
	// AvatarKey adalah key object avatar di object storage; nil bila belum
	// ada avatar.
	AvatarKey *string `gorm:"column:avatar_key"`
	// Version dipakai untuk optimistic locking (lihat Repository.UpdateWithVersion).
	Version int `gorm:"column:version;default:1"`
	// CreatedAt dan UpdatedAt disimpan sebagai epoch milidetik dan diisi
	// otomatis oleh GORM.
	CreatedAt int64 `gorm:"column:created_at;autoCreateTime:milli"`
	UpdatedAt int64 `gorm:"column:updated_at;autoCreateTime:milli;autoUpdateTime:milli"`
	// DeletedAt bernilai 0 untuk user aktif. Plugin soft_delete membuat GORM
	// otomatis menyaring user yang sudah dihapus dari setiap query.
	DeletedAt soft_delete.DeletedAt `gorm:"column:deleted_at;softDelete:milli"`
}

// TableName memberi tahu GORM nama tabel untuk entity ini.
func (u *User) TableName() string {
	return "users"
}

// GetID mengembalikan primary key.
func (u *User) GetID() string { return u.ID }

// GetVersion mengembalikan versi data untuk optimistic locking.
func (u *User) GetVersion() int { return u.Version }

// SetVersion mengubah versi data.
func (u *User) SetVersion(v int) { u.Version = v }
