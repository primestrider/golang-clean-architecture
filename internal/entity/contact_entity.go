package entity

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
}

// TableName memberi tahu GORM nama tabel untuk entity ini.
func (c *Contact) TableName() string {
	return "contacts"
}
