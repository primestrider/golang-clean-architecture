package model

// UserEvent adalah payload event perubahan user. Sengaja tidak memuat
// password maupun token.
type UserEvent struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

// GetID mengembalikan id entity sebagai key event.
func (u *UserEvent) GetID() string {
	return u.ID
}
