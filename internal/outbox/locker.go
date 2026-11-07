package outbox

import (
	"context"
	"database/sql"
	"errors"
	"sync"

	"gorm.io/gorm"
)

// MySQLLocker adalah Locker berbasis named lock MySQL (GET_LOCK).
//
// Named lock MySQL melekat pada satu koneksi database. Karena itu locker
// memegang satu koneksi khusus (*sql.Conn) selama memegang lock. Bila
// proses worker mati, koneksinya putus dan MySQL otomatis melepas lock,
// sehingga replica lain bisa mengambil alih. Tidak perlu Redis atau
// koordinator lain.
type MySQLLocker struct {
	db   *gorm.DB
	name string

	mu   sync.Mutex
	conn *sql.Conn
}

// NewMySQLLocker membuat locker untuk lock bernama name.
func NewMySQLLocker(db *gorm.DB, name string) *MySQLLocker {
	return &MySQLLocker{db: db, name: name}
}

// TryLock mengambil lock tanpa menunggu, atau memastikan lock masih
// dipegang bila sebelumnya sudah diambil.
func (l *MySQLLocker) TryLock(ctx context.Context) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.conn != nil {
		var holder sql.NullInt64
		var self int64
		err := l.conn.QueryRowContext(ctx, "SELECT IS_USED_LOCK(?), CONNECTION_ID()", l.name).Scan(&holder, &self)
		if err == nil && holder.Valid && holder.Int64 == self {
			return true, nil
		}
		// Koneksi putus atau lock hilang: buang koneksi dan coba dari awal.
		_ = l.conn.Close()
		l.conn = nil
	}

	sqlDB, err := l.db.DB()
	if err != nil {
		return false, err
	}
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return false, err
	}
	var got sql.NullInt64
	if err := conn.QueryRowContext(ctx, "SELECT GET_LOCK(?, 0)", l.name).Scan(&got); err != nil {
		_ = conn.Close()
		return false, err
	}
	if !got.Valid || got.Int64 != 1 {
		_ = conn.Close()
		return false, nil
	}
	l.conn = conn
	return true, nil
}

// Unlock melepas lock dan mengembalikan koneksinya ke pool.
func (l *MySQLLocker) Unlock(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.conn == nil {
		return nil
	}
	_, err := l.conn.ExecContext(ctx, "SELECT RELEASE_LOCK(?)", l.name)
	closeErr := l.conn.Close()
	l.conn = nil
	return errors.Join(err, closeErr)
}
