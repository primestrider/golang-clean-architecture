package model

// UserEvent adalah payload event perubahan user. Sengaja tidak memuat
// password maupun token.
type UserEvent struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	CreatedAt int64  `json:"created_at,omitempty"`
	UpdatedAt int64  `json:"updated_at,omitempty"`
}

// GetID mengembalikan id entity sebagai key event.
func (u *UserEvent) GetID() string {
	return u.ID
}
