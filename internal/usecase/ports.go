package usecase

import (
	"context"
	"io"
	"time"

	"github.com/primestrider/golang-clean-architecture/internal/entity"
	"github.com/primestrider/golang-clean-architecture/internal/model"
)

// File ini berisi "port": interface yang dibutuhkan use case dari dunia
// luar (database, message broker, dan sebagainya).
//
// Interface sengaja didefinisikan di sisi pemakai (package usecase), bukan di
// package implementasinya. Use case hanya bergantung pada perilaku yang ia
// butuhkan, implementasi konkret bisa diganti, dan unit test cukup memakai
// mock (lihat folder mocks).

// Transactor menjalankan fn di dalam satu transaksi database. Semua
// repository yang dipanggil dengan ctx dari fn ikut di transaksi yang sama.
type Transactor interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

// EventPublisher menerbitkan event domain. Dipanggil di dalam transaksi
// bisnis supaya event dan perubahan data selalu konsisten.
type EventPublisher interface {
	Publish(ctx context.Context, events ...model.Event) error
}

// UserRepository adalah akses data user yang dibutuhkan use case.
type UserRepository interface {
	// Create menyimpan user baru. Email yang sudah terdaftar menghasilkan
	// apperror CONFLICT.
	Create(ctx context.Context, user *entity.User) error
	Update(ctx context.Context, user *entity.User) error
	// UpdateWithVersion menyimpan perubahan hanya bila versi di database
	// masih expectedVersion (optimistic locking); jika tidak, CONFLICT.
	UpdateWithVersion(ctx context.Context, user *entity.User, expectedVersion int) error
	FindByID(ctx context.Context, id string) (*entity.User, error)
	FindByEmail(ctx context.Context, email string) (*entity.User, error)
	List(ctx context.Context, page, size int) ([]entity.User, int64, error)
}

// RefreshTokenRepository adalah akses data refresh token.
type RefreshTokenRepository interface {
	Create(ctx context.Context, token *entity.RefreshToken) error
	// FindByHashForUpdate mencari token berdasarkan hash dan mengunci barisnya
	// (SELECT ... FOR UPDATE) sampai transaksi selesai. Penguncian ini
	// membuat dua request refresh yang bersamaan diproses satu per satu.
	FindByHashForUpdate(ctx context.Context, hash string) (*entity.RefreshToken, error)
	// Revoke mencabut satu token dan mencatat token penggantinya (bila ada).
	Revoke(ctx context.Context, id string, replacedByID *string) error
	// RevokeFamily mencabut semua token dalam satu keluarga rotasi.
	RevokeFamily(ctx context.Context, familyID string) error
	// RevokeAllForUser mencabut semua refresh token aktif milik user,
	// misalnya setelah password diganti.
	RevokeAllForUser(ctx context.Context, userID string) error
	// DeleteExpiredBefore menghapus token yang kedaluwarsa atau dicabut
	// sebelum cutoff (epoch milidetik) dan mengembalikan jumlah yang dihapus.
	DeleteExpiredBefore(ctx context.Context, cutoff int64) (int64, error)
}

// TokenIssuer menerbitkan access token untuk user.
type TokenIssuer interface {
	Issue(userID, role string) (token string, expiresAt time.Time, err error)
}

// ContactRepository adalah akses data kontak yang dibutuhkan use case.
type ContactRepository interface {
	Create(ctx context.Context, contact *entity.Contact) error
	UpdateWithVersion(ctx context.Context, contact *entity.Contact, expectedVersion int) error
	Delete(ctx context.Context, contact *entity.Contact) error
	FindByIDAndUserID(ctx context.Context, id, userID string) (*entity.Contact, error)
	Search(ctx context.Context, req *model.SearchContactRequest) ([]entity.Contact, int64, error)
}

// AddressRepository adalah akses data alamat yang dibutuhkan use case.
type AddressRepository interface {
	Create(ctx context.Context, address *entity.Address) error
	UpdateWithVersion(ctx context.Context, address *entity.Address, expectedVersion int) error
	Delete(ctx context.Context, address *entity.Address) error
	// DeleteAllByContactID men-soft-delete semua alamat milik sebuah kontak.
	DeleteAllByContactID(ctx context.Context, contactID string) error
	FindByIDAndContactID(ctx context.Context, id, contactID string) (*entity.Address, error)
	FindAllByContactID(ctx context.Context, contactID string) ([]entity.Address, error)
}

// AuditRepository menyimpan jejak audit.
type AuditRepository interface {
	Create(ctx context.Context, log *entity.AuditLog) error
}

// OutboxRepository menyimpan dan mengelola event di tabel outbox.
type OutboxRepository interface {
	Insert(ctx context.Context, events ...entity.OutboxEvent) error
}

// Cache adalah penyimpanan sementara key-value (misalnya Redis).
type Cache interface {
	// Get mengisi dst dengan nilai key. Mengembalikan false bila key tidak ada.
	Get(ctx context.Context, key string, dst any) (bool, error)
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
	Delete(ctx context.Context, keys ...string) error
}

// ObjectStorage menyimpan file (object) di storage S3-compatible.
type ObjectStorage interface {
	Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
	Delete(ctx context.Context, key string) error
	// PresignGet membuat URL sementara untuk mengunduh object privat tanpa
	// kredensial, berlaku selama ttl.
	PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
}
