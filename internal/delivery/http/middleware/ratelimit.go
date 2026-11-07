package middleware

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	"github.com/primestrider/golang-clean-architecture/internal/apperror"
)

// NewRateLimiter membatasi jumlah request per key dalam satu jendela waktu.
//
// key menentukan siapa yang dibatasi (misalnya per IP, atau per IP+email).
// storage nil berarti hitungan disimpan di memori proses ini; bila aplikasi
// berjalan di beberapa instance, gunakan storage bersama (Redis) supaya
// batasnya berlaku global. Saat batas terlampaui, respons 429 dikirim
// dengan header Retry-After.
func NewRateLimiter(maxRequests int, window time.Duration, key func(fiber.Ctx) string, storage fiber.Storage) fiber.Handler {
	return limiter.New(limiter.Config{
		Max:          maxRequests,
		Expiration:   window,
		KeyGenerator: key,
		Storage:      storage,
		LimitReached: func(fiber.Ctx) error {
			return apperror.New(apperror.CodeRateLimited, "too many requests, please retry later")
		},
	})
}

// IPAndEmailKey membuat key rate limit dari IP client dan email di body
// JSON. Dipakai untuk endpoint login dan registrasi, supaya percobaan
// menebak password satu akun dibatasi walau penyerang berganti-ganti akun,
// dan sebaliknya.
func IPAndEmailKey(prefix string) func(fiber.Ctx) string {
	return func(c fiber.Ctx) string {
		var body struct {
			Email string `json:"email"`
		}
		_ = json.Unmarshal(c.Body(), &body)
		return prefix + ":" + c.IP() + ":" + strings.ToLower(strings.TrimSpace(body.Email))
	}
}

// IPKey membuat key rate limit dari IP client saja.
func IPKey(prefix string) func(fiber.Ctx) string {
	return func(c fiber.Ctx) string { return prefix + ":" + c.IP() }
}
