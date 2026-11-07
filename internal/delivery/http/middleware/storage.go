package middleware

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/storage/memory/v2"
)

// FailOpenStorage membungkus storage rate limiter bersama (Redis) dengan
// cadangan di memori lokal.
//
// Bila Redis tidak bisa dihubungi, operasi dialihkan ke memori lokal. API
// tetap melayani dan tetap membatasi (per instance) alih-alih menolak semua
// request dengan 500. Kegagalan dicatat sebagai warning paling sering sekali
// per menit supaya log tidak banjir.
type FailOpenStorage struct {
	primary  fiber.Storage
	fallback fiber.Storage
	log      *slog.Logger
	lastWarn atomic.Int64
}

// NewFailOpenStorage membuat FailOpenStorage di atas storage utama.
func NewFailOpenStorage(primary fiber.Storage, log *slog.Logger) *FailOpenStorage {
	return &FailOpenStorage{primary: primary, fallback: memory.New(), log: log}
}

// warn mencatat kegagalan storage utama, dibatasi sekali per menit.
func (s *FailOpenStorage) warn(err error) {
	now := time.Now().Unix()
	last := s.lastWarn.Load()
	if now-last >= 60 && s.lastWarn.CompareAndSwap(last, now) {
		s.log.Warn("rate limiter storage unavailable, using in-memory fallback", "error", err)
	}
}

// GetWithContext membaca dari storage utama, atau cadangan bila gagal.
func (s *FailOpenStorage) GetWithContext(ctx context.Context, key string) ([]byte, error) {
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
