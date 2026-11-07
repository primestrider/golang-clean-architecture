package repository

import (
	"context"

	"github.com/primestrider/golang-clean-architecture/internal/entity"
	"gorm.io/gorm"
)

// AddressRepository mengakses tabel addresses.
type AddressRepository struct {
	Repository[entity.Address]
}

// NewAddressRepository membuat AddressRepository.
func NewAddressRepository(db *gorm.DB) *AddressRepository {
	return &AddressRepository{Repository: Repository[entity.Address]{DB: db, Name: "address"}}
}

// FindByIDAndContactID mencari alamat milik kontak tertentu.
func (r *AddressRepository) FindByIDAndContactID(ctx context.Context, id, contactID string) (*entity.Address, error) {
	return r.take(r.db(ctx).Where("id = ? AND contact_id = ?", id, contactID))
}

// FindAllByContactID mengembalikan semua alamat milik sebuah kontak.
func (r *AddressRepository) FindAllByContactID(ctx context.Context, contactID string) ([]entity.Address, error) {
	var addresses []entity.Address
	if err := r.db(ctx).Where("contact_id = ?", contactID).Order("created_at").Find(&addresses).Error; err != nil {
		return nil, err
	}
	return addresses, nil
}

// DeleteAllByContactID men-soft-delete semua alamat milik sebuah kontak.
func (r *AddressRepository) DeleteAllByContactID(ctx context.Context, contactID string) error {
	return r.db(ctx).Where("contact_id = ?", contactID).Delete(&entity.Address{}).Error
}
