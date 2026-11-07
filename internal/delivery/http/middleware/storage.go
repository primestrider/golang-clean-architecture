package middleware

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/storage/memory/v2"
	"github.com/primestrider/golang-clean-architecture/internal/apperror"
)

// failoverCooldown adalah lama FailOpenStorage bertahan di cadangan setelah
// storage utama gagal, sebelum mencoba storage utama lagi.
const failoverCooldown = 30 * time.Second

// FailOpenStorage membungkus storage rate limiter bersama (Redis) dengan
// cadangan di memori lokal. Dipakai untuk limiter global (perlindungan beban),
// bukan limiter keamanan; lihat FailClosedStorage.
//
// Bila Redis tidak bisa dihubungi, operasi dialihkan ke memori lokal selama
// masa jeda (failoverCooldown), sehingga baca dan tulis tidak bercampur di
// dua storage. API tetap melayani dan tetap membatasi (per instance) alih-
// alih menolak semua request. Kegagalan dicatat paling sering sekali per menit.
type FailOpenStorage struct {
	primary   fiber.Storage
	fallback  fiber.Storage
	log       *slog.Logger
	lastWarn  atomic.Int64
	downUntil atomic.Int64
}

// NewFailOpenStorage membuat FailOpenStorage di atas storage utama.
func NewFailOpenStorage(primary fiber.Storage, log *slog.Logger) *FailOpenStorage {
	return &FailOpenStorage{primary: primary, fallback: memory.New(), log: log}
}

// usePrimary melaporkan apakah storage utama boleh dicoba (tidak sedang
// dalam masa jeda setelah gagal).
func (s *FailOpenStorage) usePrimary() bool {
	return time.Now().UnixNano() >= s.downUntil.Load()
}

// warn menandai storage utama gagal (memulai masa jeda) dan mencatatnya,
// dibatasi sekali per menit.
func (s *FailOpenStorage) warn(err error) {
	s.downUntil.Store(time.Now().Add(failoverCooldown).UnixNano())
	now := time.Now().Unix()
	last := s.lastWarn.Load()
	if now-last >= 60 && s.lastWarn.CompareAndSwap(last, now) {
		s.log.Warn("rate limiter storage unavailable, using in-memory fallback", "error", err)
	}
}

// GetWithContext membaca dari storage utama, atau cadangan bila gagal.
func (s *FailOpenStorage) GetWithContext(ctx context.Context, key string) ([]byte, error) {
	if !s.usePrimary() {
		return s.fallback.GetWithContext(ctx, key)
	}
	v, err := s.primary.GetWithContext(ctx, key)
	if err != nil {
		s.warn(err)
		return s.fallback.GetWithContext(ctx, key)
	}
	return v, nil
}

// Get membaca dari storage utama, atau cadangan bila gagal.
func (s *FailOpenStorage) Get(key string) ([]byte, error) {
	return s.GetWithContext(context.Background(), key)
}

// SetWithContext menulis ke storage utama, atau cadangan bila gagal.
func (s *FailOpenStorage) SetWithContext(ctx context.Context, key string, val []byte, exp time.Duration) error {
	if !s.usePrimary() {
		return s.fallback.SetWithContext(ctx, key, val, exp)
	}
	if err := s.primary.SetWithContext(ctx, key, val, exp); err != nil {
		s.warn(err)
		return s.fallback.SetWithContext(ctx, key, val, exp)
	}
	return nil
}

// Set menulis ke storage utama, atau cadangan bila gagal.
func (s *FailOpenStorage) Set(key string, val []byte, exp time.Duration) error {
	return s.SetWithContext(context.Background(), key, val, exp)
}

// DeleteWithContext menghapus dari kedua storage.
func (s *FailOpenStorage) DeleteWithContext(ctx context.Context, key string) error {
	_ = s.fallback.DeleteWithContext(ctx, key)
	if err := s.primary.DeleteWithContext(ctx, key); err != nil {
		s.warn(err)
	}
	return nil
}

// Delete menghapus dari kedua storage.
func (s *FailOpenStorage) Delete(key string) error {
	return s.DeleteWithContext(context.Background(), key)
}

// ResetWithContext mengosongkan kedua storage.
func (s *FailOpenStorage) ResetWithContext(ctx context.Context) error {
	_ = s.fallback.ResetWithContext(ctx)
	return s.primary.ResetWithContext(ctx)
}

// Reset mengosongkan kedua storage.
func (s *FailOpenStorage) Reset() error {
	return s.ResetWithContext(context.Background())
}

// Close menutup storage cadangan. Koneksi Redis milik aplikasi dan ditutup
// di tempat lain.
func (s *FailOpenStorage) Close() error {
	return s.fallback.Close()
}

// FailClosedStorage membungkus storage limiter keamanan (login, refresh).
// Bila Redis tidak bisa dihubungi, request ditolak dengan 503 alih-alih
// beralih ke memori lokal: penyerang tidak boleh bisa memanfaatkan gangguan
// Redis untuk melewati batas percobaan login lintas instance.
type FailClosedStorage struct {
	fiber.Storage
}

// NewFailClosedStorage membuat FailClosedStorage.
func NewFailClosedStorage(primary fiber.Storage) *FailClosedStorage {
	return &FailClosedStorage{Storage: primary}
}

// unavailable mengubah error storage menjadi apperror UNAVAILABLE (503).
func unavailable(err error) error {
	return apperror.Wrap(apperror.CodeUnavailable, "rate limiter temporarily unavailable", err)
}

// GetWithContext membaca dari storage; kegagalan menjadi 503.
func (s *FailClosedStorage) GetWithContext(ctx context.Context, key string) ([]byte, error) {
	v, err := s.Storage.GetWithContext(ctx, key)
	if err != nil {
		return nil, unavailable(err)
	}
	return v, nil
}

// Get membaca dari storage; kegagalan menjadi 503.
func (s *FailClosedStorage) Get(key string) ([]byte, error) {
	return s.GetWithContext(context.Background(), key)
}

// SetWithContext menulis ke storage; kegagalan menjadi 503.
func (s *FailClosedStorage) SetWithContext(ctx context.Context, key string, val []byte, exp time.Duration) error {
	if err := s.Storage.SetWithContext(ctx, key, val, exp); err != nil {
		return unavailable(err)
	}
	return nil
}

// Set menulis ke storage; kegagalan menjadi 503.
func (s *FailClosedStorage) Set(key string, val []byte, exp time.Duration) error {
	return s.SetWithContext(context.Background(), key, val, exp)
}
