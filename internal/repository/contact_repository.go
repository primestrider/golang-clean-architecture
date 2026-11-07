package repository

import (
	"context"

	"github.com/primestrider/golang-clean-architecture/internal/entity"
	"github.com/primestrider/golang-clean-architecture/internal/model"
	"gorm.io/gorm"
)

// ContactRepository mengakses tabel contacts.
type ContactRepository struct {
	Repository[entity.Contact]
}

// NewContactRepository membuat ContactRepository.
func NewContactRepository(db *gorm.DB) *ContactRepository {
	return &ContactRepository{Repository: Repository[entity.Contact]{DB: db, Name: "contact"}}
}

// FindByIDAndUserID mencari kontak milik user tertentu. Kontak milik user
// lain diperlakukan sama dengan kontak yang tidak ada (NOT_FOUND), supaya
// keberadaan data user lain tidak bocor.
func (r *ContactRepository) FindByIDAndUserID(ctx context.Context, id, userID string) (*entity.Contact, error) {
	return r.take(r.db(ctx).Where("id = ? AND user_id = ?", id, userID))
}

// Search mencari kontak milik user dengan filter opsional dan paging.
// Mengembalikan data halaman tersebut dan jumlah total yang cocok.
func (r *ContactRepository) Search(ctx context.Context, req *model.SearchContactRequest) ([]entity.Contact, int64, error) {
	var contacts []entity.Contact
	if err := r.db(ctx).Scopes(filterContact(req)).
		Order("created_at DESC").
		Offset((req.Page - 1) * req.Size).Limit(req.Size).
		Find(&contacts).Error; err != nil {
		return nil, 0, err
	}

	var total int64
	if err := r.db(ctx).Model(&entity.Contact{}).Scopes(filterContact(req)).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	return contacts, total, nil
}

// filterContact membangun klausa WHERE dari filter pencarian. Nilai filter
// selalu dikirim sebagai parameter query, bukan disambung ke string SQL,
// sehingga aman dari SQL injection.
func filterContact(req *model.SearchContactRequest) func(tx *gorm.DB) *gorm.DB {
	return func(tx *gorm.DB) *gorm.DB {
		tx = tx.Where("user_id = ?", req.UserID)
		if req.Name != "" {
			name := "%" + req.Name + "%"
			tx = tx.Where("(first_name LIKE ? OR last_name LIKE ?)", name, name)
		}
		if req.Phone != "" {
			tx = tx.Where("phone LIKE ?", "%"+req.Phone+"%")
		}
		if req.Email != "" {
			tx = tx.Where("email LIKE ?", "%"+req.Email+"%")
		}
		return tx
	}
}
