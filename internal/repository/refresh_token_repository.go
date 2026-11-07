package repository

import (
	"context"
	"time"

	"github.com/primestrider/golang-clean-architecture/internal/entity"
	"gorm.io/gorm"
)

// RefreshTokenRepository mengakses tabel refresh_tokens.
type RefreshTokenRepository struct {
	Repository[entity.RefreshToken]
}

// NewRefreshTokenRepository membuat RefreshTokenRepository.
func NewRefreshTokenRepository(db *gorm.DB) *RefreshTokenRepository {
	return &RefreshTokenRepository{Repository: Repository[entity.RefreshToken]{DB: db, Name: "refresh token"}}
}

// RevokeAllForUser mencabut semua refresh token milik user yang masih aktif.
func (r *RefreshTokenRepository) RevokeAllForUser(ctx context.Context, userID string) error {
	return r.db(ctx).Model(&entity.RefreshToken{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", time.Now().UnixMilli()).Error
}
