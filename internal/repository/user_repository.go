package repository

import (
	"context"
	"errors"

	"github.com/primestrider/golang-clean-architecture/internal/apperror"
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

// Create menyimpan user baru. Pelanggaran unique index email diterjemahkan
// menjadi apperror CONFLICT. Mengandalkan constraint database (bukan
// "cek dulu lalu simpan") membuat aturan email unik tetap benar walau ada
// dua registrasi bersamaan.
func (r *UserRepository) Create(ctx context.Context, user *entity.User) error {
	err := r.db(ctx).Create(user).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return apperror.Conflict("email already registered")
	}
	return err
}

// FindByEmail mencari user aktif berdasarkan email.
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*entity.User, error) {
	return r.take(r.db(ctx).Where("email = ?", email))
}

// List mengembalikan satu halaman user (terbaru lebih dulu) dan jumlah totalnya.
func (r *UserRepository) List(ctx context.Context, page, size int) ([]entity.User, int64, error) {
	var users []entity.User
	if err := r.db(ctx).Order("created_at DESC").Offset((page - 1) * size).Limit(size).Find(&users).Error; err != nil {
		return nil, 0, err
	}
	var total int64
	if err := r.db(ctx).Model(&entity.User{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

// SetAvatarKey mengubah kolom avatar_key saja dan menaikkan version, supaya
// update profil yang membawa version lama ditolak (CONFLICT).
func (r *UserRepository) SetAvatarKey(ctx context.Context, userID string, key *string) error {
	res := r.db(ctx).Model(&entity.User{}).Where("id = ?", userID).
		Updates(map[string]any{"avatar_key": key, "version": gorm.Expr("version + 1")})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return apperror.NotFound(r.Name)
	}
	return nil
}
