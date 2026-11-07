package config

import (
	"log/slog"

	"github.com/gofiber/fiber/v3"
	deliveryhttp "github.com/primestrider/golang-clean-architecture/internal/delivery/http"
)

// NewFiber membuat aplikasi Fiber dengan error handler Problem Details,
// sehingga setiap error (dari use case, middleware, atau Fiber sendiri)
// dikirim ke client dalam satu format yang konsisten.
func NewFiber(cfg *Config, log *slog.Logger) *fiber.App {
	return fiber.New(fiber.Config{
		AppName:      cfg.App.Name,
		ErrorHandler: deliveryhttp.NewErrorHandler(log),
		BodyLimit:    cfg.Server.BodyLimit,
	})
}
