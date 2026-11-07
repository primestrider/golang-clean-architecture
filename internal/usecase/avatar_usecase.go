package usecase

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/primestrider/golang-clean-architecture/internal/apperror"
	"github.com/primestrider/golang-clean-architecture/internal/entity"
	"github.com/primestrider/golang-clean-architecture/internal/model"
	"github.com/primestrider/golang-clean-architecture/internal/model/converter"
)

// MaxAvatarSize adalah ukuran maksimum file avatar (2 MB).
const MaxAvatarSize = 2 << 20

// allowedAvatarTypes memetakan tipe konten yang diizinkan ke ekstensi file.
var allowedAvatarTypes = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/webp": "webp",
}

// AvatarUseCase mengelola foto profil user yang disimpan di object storage.
type AvatarUseCase struct {
	log        *slog.Logger
	tx         Transactor
	users      UserRepository
	storage    ObjectStorage
	presignTTL time.Duration
}

// NewAvatarUseCase membuat AvatarUseCase.
func NewAvatarUseCase(log *slog.Logger, tx Transactor, users UserRepository, storage ObjectStorage, presignTTL time.Duration) *AvatarUseCase {
	return &AvatarUseCase{log: log, tx: tx, users: users, storage: storage, presignTTL: presignTTL}
}

// Upload menyimpan avatar baru untuk user.
//
// Tipe file ditentukan dari ISI file, bukan dari nama
// file atau header Content-Type kiriman client, karena keduanya bisa
// dipalsukan. Object lama baru dihapus setelah data user berhasil
// diperbarui, supaya user tidak pernah menunjuk ke file yang sudah hilang.
func (c *AvatarUseCase) Upload(ctx context.Context, userID string, file io.Reader, size int64) (*model.UserResponse, error) {
	if size > MaxAvatarSize {
		return nil, apperror.New(apperror.CodePayloadTooLarge, "avatar must be at most 2 MB")
	}

	// File dibaca utuh ke memori (maksimal 2 MB + 1 byte untuk mendeteksi
	// file yang lebih besar dari klaim ukurannya). Reader dari memori juga
	// bisa di-seek, sehingga client S3 bisa menghitung checksum lebih dulu
	// dan kompatibel dengan semua server S3.
	data, err := io.ReadAll(io.LimitReader(file, MaxAvatarSize+1))
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeBadRequest, "cannot read uploaded file", err)
	}
	if len(data) > MaxAvatarSize {
		return nil, apperror.New(apperror.CodePayloadTooLarge, "avatar must be at most 2 MB")
	}
	contentType := http.DetectContentType(data)
	ext, ok := allowedAvatarTypes[contentType]
	if !ok {
		return nil, apperror.New(apperror.CodeUnsupportedMedia, "avatar must be a JPEG, PNG, or WebP image")
	}

	key := "avatars/" + userID + "/" + uuid.NewString() + "." + ext
	if err := c.storage.Put(ctx, key, bytes.NewReader(data), int64(len(data)), contentType); err != nil {
		return nil, apperror.Internal(err)
	}

	var user *entity.User
	var oldKey *string
	err = c.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		var err error
		user, err = c.users.FindByID(ctx, userID)
		if err != nil {
			return err
		}
		oldKey = user.AvatarKey
		if err := c.users.SetAvatarKey(ctx, userID, &key); err != nil {
			return err
		}
		user, err = c.users.FindByID(ctx, userID)
		return err
	})
	if err != nil {
		// Data user gagal diperbarui: buang object yang baru diunggah.
		c.deleteObject(ctx, key)
		return nil, err
	}
	if oldKey != nil {
		c.deleteObject(ctx, *oldKey)
	}
	return c.Response(ctx, user), nil
}

// Remove menghapus avatar user.
func (c *AvatarUseCase) Remove(ctx context.Context, userID string) error {
	var oldKey *string
	err := c.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		user, err := c.users.FindByID(ctx, userID)
		if err != nil {
			return err
		}
		oldKey = user.AvatarKey
		return c.users.SetAvatarKey(ctx, userID, nil)
	})
	if err != nil {
		return err
	}
	if oldKey != nil {
		c.deleteObject(ctx, *oldKey)
	}
	return nil
}

// Response mengubah user menjadi respons API lengkap dengan presigned URL
// avatar. Bucket tetap privat; URL hanya berlaku selama presign_ttl.
func (c *AvatarUseCase) Response(ctx context.Context, user *entity.User) *model.UserResponse {
	res := converter.UserToResponse(user)
	if user.AvatarKey == nil {
		return res
	}
	url, err := c.storage.PresignGet(ctx, *user.AvatarKey, c.presignTTL)
	if err != nil {
		c.log.WarnContext(ctx, "presign avatar url failed", "error", err)
		return res
	}
	res.AvatarURL = url
	return res
}

// deleteObject menghapus object dan hanya mencatat kegagalannya; object
// yatim tidak merusak data dan bisa dibersihkan belakangan.
func (c *AvatarUseCase) deleteObject(ctx context.Context, key string) {
	if err := c.storage.Delete(ctx, key); err != nil {
		c.log.WarnContext(ctx, "delete avatar object failed", "key", key, "error", err)
	}
}
