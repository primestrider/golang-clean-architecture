package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/primestrider/golang-clean-architecture/internal/apperror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// brokenStorage meniru Redis yang mati: semua operasi gagal.
type brokenStorage struct{}

var errDown = errors.New("redis: connection refused")

func (brokenStorage) GetWithContext(context.Context, string) ([]byte, error) { return nil, errDown }
func (brokenStorage) Get(string) ([]byte, error)                             { return nil, errDown }
func (brokenStorage) SetWithContext(context.Context, string, []byte, time.Duration) error {
	return errDown
}
func (brokenStorage) Set(string, []byte, time.Duration) error         { return errDown }
func (brokenStorage) DeleteWithContext(context.Context, string) error { return errDown }
func (brokenStorage) Delete(string) error                             { return errDown }
func (brokenStorage) ResetWithContext(context.Context) error          { return errDown }
func (brokenStorage) Reset() error                                    { return errDown }
func (brokenStorage) Close() error                                    { return nil }

// TestRateLimiter_FailsOpenWhenStorageIsDown: Redis yang mati tidak boleh
// membuat seluruh API error. Limiter beralih ke memori lokal.
func TestRateLimiter_FailsOpenWhenStorageIsDown(t *testing.T) {
	storage := NewFailOpenStorage(brokenStorage{}, slog.New(slog.DiscardHandler))
	// Error handler minimal yang memetakan RATE_LIMITED ke 429, seperti
	// error handler aplikasi.
	app := fiber.New(fiber.Config{ErrorHandler: func(c fiber.Ctx, err error) error {
		if appErr, ok := apperror.As(err); ok && appErr.Code == apperror.CodeRateLimited {
			return c.SendStatus(fiber.StatusTooManyRequests)
		}
		return c.SendStatus(fiber.StatusInternalServerError)
	}})
	app.Use(NewRateLimiter(2, time.Minute, IPKey("t"), storage))
	app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	var codes []int
	for range 3 {
		resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
		require.NoError(t, err)
		_ = resp.Body.Close()
		codes = append(codes, resp.StatusCode)
	}

	assert.Equal(t, []int{200, 200, 429}, codes, "still serving and still limiting via the in-memory fallback")
}

// TestFailClosedStorage_ReturnsUnavailable: limiter keamanan (login,
// refresh) tidak boleh beralih ke memori lokal saat Redis mati, karena
// penyerang bisa memanfaatkan gangguan itu untuk melewati batas lintas
// instance. Request ditolak 503.
func TestFailClosedStorage_ReturnsUnavailable(t *testing.T) {
	_, err := NewFailClosedStorage(brokenStorage{}).GetWithContext(context.Background(), "k")

	appErr, ok := apperror.As(err)
	require.True(t, ok)
	assert.Equal(t, apperror.CodeUnavailable, appErr.Code)
}

// TestFailOpenStorage_StaysOnFallbackDuringCooldown: setelah storage utama
// gagal, operasi berikutnya langsung memakai cadangan selama masa jeda,
// supaya Get dan Set tidak bercampur di dua storage berbeda.
func TestFailOpenStorage_StaysOnFallbackDuringCooldown(t *testing.T) {
	primary := &countingStorage{fail: true}
	s := NewFailOpenStorage(primary, slog.New(slog.DiscardHandler))

	_, _ = s.GetWithContext(context.Background(), "k")
	primary.fail = false
	_ = s.SetWithContext(context.Background(), "k", []byte("v"), time.Minute)

	assert.Equal(t, 1, primary.calls, "primary must not be retried during the cooldown")
}

// countingStorage menghitung pemanggilan dan bisa disetel gagal.
type countingStorage struct {
	brokenStorage
	fail  bool
	calls int
}

func (c *countingStorage) GetWithContext(context.Context, string) ([]byte, error) {
	c.calls++
	if c.fail {
		return nil, errDown
	}
	return nil, nil
}

func (c *countingStorage) SetWithContext(context.Context, string, []byte, time.Duration) error {
	c.calls++
	if c.fail {
		return errDown
	}
	return nil
}
