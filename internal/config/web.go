package config

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/primestrider/golang-clean-architecture/internal/delivery/http"
	"github.com/primestrider/golang-clean-architecture/internal/delivery/http/middleware"
	"github.com/primestrider/golang-clean-architecture/internal/delivery/http/route"
	"github.com/primestrider/golang-clean-architecture/internal/observability"
	"github.com/primestrider/golang-clean-architecture/internal/repository"
	"github.com/primestrider/golang-clean-architecture/internal/usecase"
	"github.com/primestrider/golang-clean-architecture/internal/validation"
	"gorm.io/gorm"
)

// WebDeps adalah dependency tingkat infrastruktur yang dibutuhkan aplikasi
// web. Dibuat di main lalu diteruskan ke BootstrapWeb.
type WebDeps struct {
	Config *Config
	Log    *slog.Logger
	DB     *gorm.DB
	App    *fiber.App
	// Metrics mencatat metrik HTTP; wajib diisi.
	Metrics *observability.Metrics
}

// BootstrapWeb merangkai seluruh aplikasi web: repository, use case,
// controller, middleware, dan route. Urutannya mengikuti arah dependency,
// dari layer paling dalam (repository) ke paling luar (route).
func BootstrapWeb(deps *WebDeps) {
	// Repository dan transactor: akses data.
	tx := repository.NewTransactor(deps.DB)
	userRepository := repository.NewUserRepository(deps.DB)
	contactRepository := repository.NewContactRepository(deps.DB)
	addressRepository := repository.NewAddressRepository(deps.DB)
	refreshTokenRepository := repository.NewRefreshTokenRepository(deps.DB)
	audit := usecase.NewAuditRecorder(repository.NewAuditRepository(deps.DB))

	// Penerbit dan pemverifikasi access token JWT.
	tokens := NewTokenIssuer(deps.Config)

	// Publisher event: event ditulis ke tabel outbox di transaksi bisnis.
	events := usecase.NewOutboxPublisher(repository.NewOutboxRepository(deps.DB))

	// Use case: logika bisnis.
	validate := validation.New()
	userUseCase := usecase.NewUserUseCase(deps.Log, validate, tx, userRepository, refreshTokenRepository, events, audit)
	contactUseCase := usecase.NewContactUseCase(deps.Log, validate, tx, contactRepository, addressRepository, events, audit)
	authUseCase := usecase.NewAuthUseCase(usecase.AuthDeps{
		Log:        deps.Log,
		Validate:   validate,
		Tx:         tx,
		Users:      userRepository,
		Tokens:     refreshTokenRepository,
		Issuer:     tokens,
		Audit:      audit,
		RefreshTTL: deps.Config.JWT.RefreshTTL,
	})
	adminUseCase := usecase.NewAdminUseCase(deps.Log, validate, tx, userRepository, events, audit)
	addressUseCase := usecase.NewAddressUseCase(deps.Log, validate, tx, contactRepository, addressRepository, events, audit)

	// Delivery: middleware global, lalu controller dan route.
	middleware.Apply(deps.App, middleware.StackConfig{
		CORSOrigins:     deps.Config.Server.CORSOrigins,
		RequestTimeout:  deps.Config.Server.RequestTimeout,
		GlobalPerMinute: deps.Config.RateLimit.GlobalPerMinute,
		Extra:           []fiber.Handler{deps.Metrics.HTTPMiddleware()},
	}, deps.Log)

	authLimit := deps.Config.RateLimit.AuthPerMinute
	routes := route.RouteConfig{
		App:                  deps.App,
		AuthController:       http.NewAuthController(authUseCase),
		UserController:       http.NewUserController(userUseCase),
		ContactController:    http.NewContactController(contactUseCase),
		AddressController:    http.NewAddressController(addressUseCase),
		AdminController:      http.NewAdminController(adminUseCase),
		AuthMiddleware:       middleware.NewAuth(tokens),
		AuthIPEmailRateLimit: middleware.NewRateLimiter(authLimit, time.Minute, middleware.IPAndEmailKey("auth-ip-email"), nil),
		AuthIPRateLimit:      middleware.NewRateLimiter(deps.Config.RateLimit.AuthIPPerMinute, time.Minute, middleware.IPKey("auth-ip"), nil),
		AuthEmailRateLimit: middleware.NewFailureRateLimiter(deps.Config.RateLimit.AuthEmailFailuresPerMinute, time.Minute,
			middleware.EmailKey("auth-email"), nil),
		RefreshRateLimit: middleware.NewRateLimiter(authLimit, time.Minute, middleware.IPKey("refresh"), nil),
	}
	routes.Setup()
}
