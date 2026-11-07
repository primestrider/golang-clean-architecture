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
