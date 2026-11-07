package model

// ContactEvent adalah payload event perubahan kontak.
type ContactEvent struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

// GetID mengembalikan id entity sebagai key event.
func (c *ContactEvent) GetID() string {
	return c.ID
}
