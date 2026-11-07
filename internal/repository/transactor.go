package repository

import (
	"context"

	"gorm.io/gorm"
)

// txKey adalah kunci context privat untuk menyimpan transaksi GORM aktif.
type txKey struct{}

// GormTransactor menjalankan fungsi di dalam transaksi database.
//
// Polanya: transaksi disimpan di context.Context, lalu setiap repository
// mengambilnya lewat DB(ctx, base). Dengan begitu use case cukup menulis
//
//	tx.WithinTransaction(ctx, func(ctx context.Context) error {
//	    ... panggil beberapa repository dengan ctx ...
//	})
//
// tanpa pernah menyentuh *gorm.DB. Use case tetap bebas dari detail database
// dan mudah di-unit-test dengan transactor palsu.
type GormTransactor struct {
	db *gorm.DB
}

// NewTransactor membuat GormTransactor di atas koneksi database utama.
func NewTransactor(db *gorm.DB) *GormTransactor {
	return &GormTransactor{db: db}
}

// WithinTransaction menjalankan fn di dalam transaksi. Transaksi di-commit
// bila fn mengembalikan nil, dan di-rollback bila fn mengembalikan error
// atau panic.
//
// Jika context sudah membawa transaksi, fn ikut di transaksi yang sama
// (tidak membuat transaksi bersarang), sehingga use case boleh saling
// memanggil tanpa memecah atomisitas.
func (t *GormTransactor) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return fn(ctx)
	}
	return t.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(context.WithValue(ctx, txKey{}, tx))
	})
}

// DB mengembalikan transaksi aktif dari context bila ada; jika tidak,
// mengembalikan koneksi biasa yang terikat ke ctx. Operasi baca di luar
// transaksi tidak perlu membuka transaksi sama sekali.
func DB(ctx context.Context, base *gorm.DB) *gorm.DB {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return tx
	}
	return base.WithContext(ctx)
}
