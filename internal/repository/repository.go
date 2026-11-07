package repository

import (
	"context"
	"errors"

	"github.com/primestrider/golang-clean-architecture/internal/apperror"
	"gorm.io/gorm"
)

// Repository adalah repository generik berisi operasi CRUD dasar untuk
// entity T. Repository spesifik (UserRepository, ContactRepository, ...)
// meng-embed struct ini lalu menambah query khusus miliknya.
type Repository[T any] struct {
	// DB adalah koneksi utama. Jangan dipakai langsung di query; selalu
	// lewat r.db(ctx) supaya transaksi dari context ikut terpakai.
	DB *gorm.DB
	// Name adalah nama entity untuk pesan error, misalnya "contact".
	Name string
}

// db mengembalikan koneksi yang tepat untuk ctx (transaksi atau biasa).
func (r *Repository[T]) db(ctx context.Context) *gorm.DB {
	return DB(ctx, r.DB)
}

// Create menyimpan entity baru.
func (r *Repository[T]) Create(ctx context.Context, entity *T) error {
	return r.db(ctx).Create(entity).Error
}

// Update menyimpan seluruh field entity.
func (r *Repository[T]) Update(ctx context.Context, entity *T) error {
	return r.db(ctx).Save(entity).Error
}

// Delete menghapus entity.
func (r *Repository[T]) Delete(ctx context.Context, entity *T) error {
	return r.db(ctx).Delete(entity).Error
}

// versioned adalah entity yang mendukung optimistic locking.
type versioned interface {
	GetID() string
	GetVersion() int
	SetVersion(v int)
}

// UpdateWithVersion menyimpan perubahan entity hanya bila versinya di
// database masih expectedVersion, lalu menaikkan versinya.
//
// Ini adalah optimistic locking: dua orang yang mengedit data yang sama
// berdasarkan versi yang sama tidak akan saling menimpa diam-diam. Yang kedua
// mendapat CONFLICT dan harus memuat ulang data terbaru. Bila data sudah
// tidak ada (termasuk sudah di-soft-delete), hasilnya NOT_FOUND.
//
// Query yang dihasilkan kurang lebih:
//
//	UPDATE ... SET ..., version = expected+1 WHERE id = ? AND version = expected AND deleted_at = 0
func (r *Repository[T]) UpdateWithVersion(ctx context.Context, entity *T, expectedVersion int) error {
	v, ok := any(entity).(versioned)
	if !ok {
		return apperror.Internal(errors.New(r.Name + " does not support versioning"))
	}
	v.SetVersion(expectedVersion + 1)
	res := r.db(ctx).Model(entity).Where("version = ?", expectedVersion).Select("*").Updates(entity)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 1 {
		return nil
	}

	// Tidak ada baris yang berubah: bedakan "data sudah tidak ada" dengan
	// "versinya sudah berubah".
	v.SetVersion(expectedVersion)
	var count int64
	if err := r.db(ctx).Model(new(T)).Where("id = ?", v.GetID()).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return apperror.NotFound(r.Name)
	}
	return apperror.Conflict(r.Name + " was modified by another request, reload and try again")
}

// FindByID mencari entity berdasarkan primary key. Mengembalikan
// apperror NOT_FOUND bila tidak ada.
func (r *Repository[T]) FindByID(ctx context.Context, id string) (*T, error) {
	return r.take(r.db(ctx).Where("id = ?", id))
}

// take menjalankan query yang mengembalikan satu baris dan menerjemahkan
// gorm.ErrRecordNotFound menjadi apperror NOT_FOUND. Error lain dikembalikan
// apa adanya dan akan menjadi 500 di delivery.
func (r *Repository[T]) take(query *gorm.DB) (*T, error) {
	entity := new(T)
	if err := query.Take(entity).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.NotFound(r.Name)
		}
		return nil, err
	}
	return entity, nil
}
