package entity

// User adalah pemilik akun. Setiap user memiliki banyak Contact.
type User struct {
	ID string `gorm:"column:id;primaryKey"`
	// Password berisi hash bcrypt, tidak pernah password asli.
	Password string `gorm:"column:password"`
	Name     string `gorm:"column:name"`
	Token    string `gorm:"column:token"`
	// CreatedAt dan UpdatedAt disimpan sebagai epoch milidetik dan diisi
	// otomatis oleh GORM.
	CreatedAt int64 `gorm:"column:created_at;autoCreateTime:milli"`
	UpdatedAt int64 `gorm:"column:updated_at;autoCreateTime:milli;autoUpdateTime:milli"`
}

// TableName memberi tahu GORM nama tabel untuk entity ini.
func (u *User) TableName() string {
	return "users"
}
