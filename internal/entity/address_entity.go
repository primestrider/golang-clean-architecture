package entity

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
}

// TableName memberi tahu GORM nama tabel untuk entity ini.
func (a *Address) TableName() string {
	return "addresses"
}
