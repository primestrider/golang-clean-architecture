package model

// AddressResponse adalah representasi alamat yang dikirim ke client.
type AddressResponse struct {
	ID         string `json:"id"`
	Street     string `json:"street"`
	City       string `json:"city"`
	Province   string `json:"province"`
	PostalCode string `json:"postal_code"`
	Country    string `json:"country"`
	CreatedAt  int64  `json:"created_at"`
	UpdatedAt  int64  `json:"updated_at"`
}

// ListAddressRequest meminta semua alamat milik sebuah kontak.
type ListAddressRequest struct {
	UserID    string `json:"-" validate:"required"`
	ContactID string `json:"-" validate:"required,max=100,uuid"`
}

// CreateAddressRequest adalah input pembuatan alamat.
type CreateAddressRequest struct {
	UserID     string `json:"-" validate:"required"`
	ContactID  string `json:"-" validate:"required,max=100,uuid"`
	Street     string `json:"street" validate:"max=255"`
	City       string `json:"city" validate:"max=255"`
	Province   string `json:"province" validate:"max=255"`
	PostalCode string `json:"postal_code" validate:"max=10"`
	Country    string `json:"country" validate:"max=100"`
}

// UpdateAddressRequest adalah input pengubahan alamat.
type UpdateAddressRequest struct {
	UserID     string `json:"-" validate:"required"`
	ContactID  string `json:"-" validate:"required,max=100,uuid"`
	ID         string `json:"-" validate:"required,max=100,uuid"`
	Street     string `json:"street" validate:"max=255"`
	City       string `json:"city" validate:"max=255"`
	Province   string `json:"province" validate:"max=255"`
	PostalCode string `json:"postal_code" validate:"max=10"`
	Country    string `json:"country" validate:"max=100"`
}

// GetAddressRequest mengidentifikasi satu alamat.
type GetAddressRequest struct {
	UserID    string `json:"-" validate:"required"`
	ContactID string `json:"-" validate:"required,max=100,uuid"`
	ID        string `json:"-" validate:"required,max=100,uuid"`
}

// DeleteAddressRequest mengidentifikasi alamat yang akan dihapus.
type DeleteAddressRequest struct {
	UserID    string `json:"-" validate:"required"`
	ContactID string `json:"-" validate:"required,max=100,uuid"`
	ID        string `json:"-" validate:"required,max=100,uuid"`
}
