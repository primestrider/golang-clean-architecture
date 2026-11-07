package config

import (
	"log/slog"

	"github.com/gofiber/fiber/v3"
	deliveryhttp "github.com/primestrider/golang-clean-architecture/internal/delivery/http"
)

// NewFiber membuat aplikasi Fiber dengan error handler Problem Details,
// sehingga setiap error (dari use case, middleware, atau Fiber sendiri)
// dikirim ke client dalam satu format yang konsisten.
//
// Header X-Forwarded-For hanya dipercaya bila request datang dari proxy di
// server.trusted_proxies. Tanpa itu, client bisa memalsukan IP-nya sendiri
// dan mengakali rate limit.
func NewFiber(cfg *Config, log *slog.Logger) *fiber.App {
	trusted := len(cfg.Server.TrustedProxies) > 0
	return fiber.New(fiber.Config{
		AppName:          cfg.App.Name,
		ErrorHandler:     deliveryhttp.NewErrorHandler(log),
		BodyLimit:        cfg.Server.BodyLimit,
		ReadTimeout:      cfg.Server.RequestTimeout,
		WriteTimeout:     cfg.Server.RequestTimeout,
		TrustProxy:       trusted,
		TrustProxyConfig: fiber.TrustProxyConfig{Proxies: cfg.Server.TrustedProxies},
		ProxyHeader:      proxyHeader(trusted),
	})
}

// proxyHeader mengembalikan header IP asli client bila ada proxy tepercaya.
func proxyHeader(trusted bool) string {
	if trusted {
		return fiber.HeaderXForwardedFor
	}
	return ""
}
