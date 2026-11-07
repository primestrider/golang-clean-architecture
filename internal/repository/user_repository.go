package repository

import (
	"context"

	"github.com/primestrider/golang-clean-architecture/internal/entity"
	"gorm.io/gorm"
)

// UserRepository mengakses tabel users.
type UserRepository struct {
	Repository[entity.User]
}

// NewUserRepository membuat UserRepository.
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{Repository: Repository[entity.User]{DB: db, Name: "user"}}
}

// FindByToken mencari user berdasarkan token sesi.
func (r *UserRepository) FindByToken(ctx context.Context, token string) (*entity.User, error) {
	return r.take(r.db(ctx).Where("token = ?", token))
}
