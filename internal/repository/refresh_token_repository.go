package repository

import (
	"context"
	"time"

	"github.com/primestrider/golang-clean-architecture/internal/entity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// RefreshTokenRepository mengakses tabel refresh_tokens.
type RefreshTokenRepository struct {
	Repository[entity.RefreshToken]
}

// NewRefreshTokenRepository membuat RefreshTokenRepository.
func NewRefreshTokenRepository(db *gorm.DB) *RefreshTokenRepository {
	return &RefreshTokenRepository{Repository: Repository[entity.RefreshToken]{DB: db, Name: "refresh token"}}
}

// FindByHashForUpdate mencari token berdasarkan hash dan mengunci barisnya
// (SELECT ... FOR UPDATE). Harus dipanggil di dalam transaksi; kunci dilepas
// saat transaksi selesai.
func (r *RefreshTokenRepository) FindByHashForUpdate(ctx context.Context, hash string) (*entity.RefreshToken, error) {
	return r.take(r.db(ctx).Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate}).Where("token_hash = ?", hash))
}

// Revoke mencabut satu token dan mencatat token penggantinya.
func (r *RefreshTokenRepository) Revoke(ctx context.Context, id string, replacedByID *string) error {
	return r.db(ctx).Model(&entity.RefreshToken{}).Where("id = ?", id).
		Updates(map[string]any{"revoked_at": time.Now().UnixMilli(), "replaced_by_id": replacedByID}).Error
}

// RevokeFamily mencabut semua token aktif dalam satu keluarga rotasi.
func (r *RefreshTokenRepository) RevokeFamily(ctx context.Context, familyID string) error {
	return r.db(ctx).Model(&entity.RefreshToken{}).
		Where("family_id = ? AND revoked_at IS NULL", familyID).
		Update("revoked_at", time.Now().UnixMilli()).Error
}

// RevokeAllForUser mencabut semua refresh token milik user yang masih aktif.
func (r *RefreshTokenRepository) RevokeAllForUser(ctx context.Context, userID string) error {
	return r.db(ctx).Model(&entity.RefreshToken{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", time.Now().UnixMilli()).Error
}

// DeleteExpiredBefore menghapus token yang sudah kedaluwarsa atau dicabut
// sebelum cutoff (epoch milidetik). Dipakai job pembersihan terjadwal.
func (r *RefreshTokenRepository) DeleteExpiredBefore(ctx context.Context, cutoff int64) (int64, error) {
	res := r.db(ctx).Where("expires_at < ? OR revoked_at < ?", cutoff, cutoff).Delete(&entity.RefreshToken{})
	return res.RowsAffected, res.Error
}
