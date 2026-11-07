package model

// AddressEvent adalah payload event perubahan alamat.
type AddressEvent struct {
	ID         string `json:"id"`
	ContactID  string `json:"contact_id"`
	Street     string `json:"street"`
	City       string `json:"city"`
	Province   string `json:"province"`
	PostalCode string `json:"postal_code"`
	Country    string `json:"country"`
	CreatedAt  int64  `json:"created_at"`
	UpdatedAt  int64  `json:"updated_at"`
}

// GetID mengembalikan id entity sebagai key event.
func (a *AddressEvent) GetID() string {
	return a.ID
}
