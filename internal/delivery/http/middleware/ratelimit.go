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
// Semua request dihitung, berhasil maupun gagal.
//
// key menentukan siapa yang dibatasi (misalnya per IP, atau per IP+email).
// storage nil berarti hitungan disimpan di memori proses ini; bila aplikasi
// berjalan di beberapa instance, gunakan storage bersama (Redis) supaya
// batasnya berlaku global. Saat batas terlampaui, respons 429 dikirim
// dengan header Retry-After.
func NewRateLimiter(maxRequests int, window time.Duration, key func(fiber.Ctx) string, storage fiber.Storage) fiber.Handler {
	return newLimiter(maxRequests, window, key, storage, false)
}

// NewFailureRateLimiter sama seperti NewRateLimiter, tetapi hanya menghitung
// request yang gagal (status >= 400). Cocok untuk limiter per akun: login
// yang berhasil tidak pernah membuat pemilik akun terkunci.
func NewFailureRateLimiter(maxFailures int, window time.Duration, key func(fiber.Ctx) string, storage fiber.Storage) fiber.Handler {
	return newLimiter(maxFailures, window, key, storage, true)
}

// newLimiter membangun limiter Fiber dengan respons 429 berformat Problem Details.
func newLimiter(maxRequests int, window time.Duration, key func(fiber.Ctx) string, storage fiber.Storage, skipSuccessful bool) fiber.Handler {
	return limiter.New(limiter.Config{
		SkipSuccessfulRequests: skipSuccessful,
		Max:                    maxRequests,
		Expiration:             window,
		KeyGenerator:           key,
		Storage:                storage,
		LimitReached: func(fiber.Ctx) error {
			return apperror.New(apperror.CodeRateLimited, "too many requests, please retry later")
		},
	})
}

// EmailKey membuat key rate limit dari email di body JSON. Dipakai bersama
// NewFailureRateLimiter untuk membatasi tebakan password terdistribusi ke
// satu akun dari banyak IP.
func EmailKey(prefix string) func(fiber.Ctx) string {
	return func(c fiber.Ctx) string { return prefix + ":" + emailFromBody(c) }
}

// IPAndEmailKey membuat key rate limit dari IP client dan email di body.
// Membatasi satu sumber yang menebak password satu akun.
func IPAndEmailKey(prefix string) func(fiber.Ctx) string {
	return func(c fiber.Ctx) string { return prefix + ":" + c.IP() + ":" + emailFromBody(c) }
}

// emailFromBody membaca field email dari body JSON dalam bentuk ternormalisasi.
func emailFromBody(c fiber.Ctx) string {
	var body struct {
		Email string `json:"email"`
	}
	_ = json.Unmarshal(c.Body(), &body)
	return strings.ToLower(strings.TrimSpace(body.Email))
}

// IPKey membuat key rate limit dari IP client saja.
func IPKey(prefix string) func(fiber.Ctx) string {
	return func(c fiber.Ctx) string { return prefix + ":" + c.IP() }
}

// RenderErrors langsung merender error dari handler berikutnya lewat error
// handler aplikasi, sehingga status respons sudah terisi ketika middleware
// di depannya (misalnya NewFailureRateLimiter) memeriksa apakah request
// gagal. Pasang tepat sebelum controller.
func RenderErrors() fiber.Handler {
	return func(c fiber.Ctx) error {
		if err := c.Next(); err != nil {
			return c.App().ErrorHandler(c, err)
		}
		return nil
	}
}
