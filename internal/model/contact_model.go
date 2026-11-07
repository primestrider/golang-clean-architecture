package model

// ContactResponse adalah representasi kontak yang dikirim ke client.
type ContactResponse struct {
	ID        string            `json:"id"`
	FirstName string            `json:"first_name"`
	LastName  string            `json:"last_name"`
	Email     string            `json:"email"`
	Phone     string            `json:"phone"`
	Version   int               `json:"version"`
	CreatedAt int64             `json:"created_at"`
	UpdatedAt int64             `json:"updated_at"`
	Addresses []AddressResponse `json:"addresses,omitempty"`
}

// CreateContactRequest adalah input pembuatan kontak.
//
// Field bertag json:"-" (seperti UserID) tidak pernah dibaca dari body.
// Nilainya diisi controller dari token login, sehingga client tidak bisa
// membuat kontak atas nama user lain.
type CreateContactRequest struct {
	UserID    string `json:"-" validate:"required"`
	FirstName string `json:"first_name" validate:"required,max=100"`
	LastName  string `json:"last_name" validate:"max=100"`
	Email     string `json:"email" validate:"omitempty,max=200,email"`
	Phone     string `json:"phone" validate:"max=20"`
}

// UpdateContactRequest adalah input pengubahan kontak.
type UpdateContactRequest struct {
	UserID    string `json:"-" validate:"required"`
	ID        string `json:"-" validate:"required,max=100,uuid"`
	FirstName string `json:"first_name" validate:"required,max=100"`
	LastName  string `json:"last_name" validate:"max=100"`
	Email     string `json:"email" validate:"omitempty,max=200,email"`
	Phone     string `json:"phone" validate:"max=20"`
	// Version adalah versi data yang terakhir dilihat client (optimistic locking).
	Version int `json:"version" validate:"required,min=1"`
}

// SearchContactRequest adalah filter dan paging untuk daftar kontak.
type SearchContactRequest struct {
	UserID string `json:"-" validate:"required"`
	Name   string `json:"name" validate:"max=100"`
	Email  string `json:"email" validate:"max=200"`
	Phone  string `json:"phone" validate:"max=20"`
	Page   int    `json:"page" validate:"min=1"`
	Size   int    `json:"size" validate:"min=1,max=100"`
}

// GetContactRequest mengidentifikasi satu kontak milik user.
type GetContactRequest struct {
	UserID string `json:"-" validate:"required"`
	ID     string `json:"-" validate:"required,max=100,uuid"`
}

// DeleteContactRequest mengidentifikasi kontak yang akan dihapus.
type DeleteContactRequest struct {
	UserID string `json:"-" validate:"required"`
	ID     string `json:"-" validate:"required,max=100,uuid"`
}
