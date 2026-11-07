package config

import (
	"log/slog"
	"time"

	fiberotel "github.com/gofiber/contrib/v3/otel"
	"github.com/gofiber/fiber/v3"
	redisstorage "github.com/gofiber/storage/redis/v3"
	"github.com/primestrider/golang-clean-architecture/api"
	"github.com/primestrider/golang-clean-architecture/internal/delivery/http"
	"github.com/primestrider/golang-clean-architecture/internal/delivery/http/docs"
	"github.com/primestrider/golang-clean-architecture/internal/delivery/http/middleware"
	"github.com/primestrider/golang-clean-architecture/internal/delivery/http/route"
	"github.com/primestrider/golang-clean-architecture/internal/gateway/cache"
	"github.com/primestrider/golang-clean-architecture/internal/observability"
	"github.com/primestrider/golang-clean-architecture/internal/repository"
	"github.com/primestrider/golang-clean-architecture/internal/security"
	"github.com/primestrider/golang-clean-architecture/internal/usecase"
	"github.com/primestrider/golang-clean-architecture/internal/validation"
	"github.com/redis/go-redis/v9"
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
	// Redis opsional (nil bila dimatikan): dipakai untuk cache dan storage
	// rate limiter bersama antar instance.
	Redis *redis.Client
	// Storage opsional (nil bila dimatikan) untuk avatar.
	Storage usecase.ObjectStorage
}

// WebComponents adalah bagian aplikasi web yang juga dipakai delivery lain
// (misalnya server gRPC), supaya keduanya memakai use case yang sama.
type WebComponents struct {
	Contacts *usecase.ContactUseCase
	Tokens   *security.TokenIssuer
	Metrics  *observability.Metrics
}

// BootstrapWeb merangkai seluruh aplikasi web: repository, use case,
// controller, middleware, dan route. Urutannya mengikuti arah dependency,
// dari layer paling dalam (repository) ke paling luar (route).
func BootstrapWeb(deps *WebDeps) *WebComponents {
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

	// Cache dan storage rate limiter: Redis bila aktif, selain itu memori.
	var contactCache usecase.Cache = cache.NoopCache{}
	var limiterStorage fiber.Storage
	if deps.Redis != nil {
		contactCache = cache.NewRedisCache(deps.Redis)
		// Fail-open: bila Redis mati, limiter beralih ke memori lokal alih-alih
		// membuat semua request gagal.
		limiterStorage = middleware.NewFailOpenStorage(redisstorage.NewFromConnection(deps.Redis), deps.Log)
	}

	// Use case: logika bisnis.
	validate := validation.New()
	userUseCase := usecase.NewUserUseCase(deps.Log, validate, tx, userRepository, refreshTokenRepository, events, audit)
	contactUseCase := usecase.NewContactUseCase(deps.Log, validate, tx, contactRepository, addressRepository, events, audit, contactCache)
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

	// Avatar hanya tersedia bila object storage aktif.
	var avatarController *http.AvatarController
	if deps.Storage != nil {
		avatarUseCase := usecase.NewAvatarUseCase(deps.Log, tx, userRepository, deps.Storage, deps.Config.Storage.PresignTTL)
		userUseCase.UseAvatarURLs(avatarUseCase)
		avatarController = http.NewAvatarController(avatarUseCase)
	}

	// Delivery: middleware global, lalu controller dan route.
	middleware.Apply(deps.App, middleware.StackConfig{
		CORSOrigins:     deps.Config.Server.CORSOrigins,
		RequestTimeout:  deps.Config.Server.RequestTimeout,
		GlobalPerMinute: deps.Config.RateLimit.GlobalPerMinute,
		Storage:         limiterStorage,
		JSONBodyLimit:   deps.Config.Server.BodyLimit,
		// Tracing lebih dulu supaya span mencakup seluruh pemrosesan request.
		// Filter pencarian di query string adalah data pribadi, jadi nilainya
		// disamarkan di atribut span.
		Extra: []fiber.Handler{
			fiberotel.New(fiberotel.WithRedactedQueryParams("name", "email", "phone")),
			deps.Metrics.HTTPMiddleware(),
		},
	}, deps.Log)

	authLimit := deps.Config.RateLimit.AuthPerMinute
	routes := route.RouteConfig{
		App:                  deps.App,
		AuthController:       http.NewAuthController(authUseCase),
		UserController:       http.NewUserController(userUseCase),
		ContactController:    http.NewContactController(contactUseCase),
		AddressController:    http.NewAddressController(addressUseCase),
		AdminController:      http.NewAdminController(adminUseCase),
		AvatarController:     avatarController,
		AuthMiddleware:       middleware.NewAuth(tokens),
		AuthIPEmailRateLimit: middleware.NewRateLimiter(authLimit, time.Minute, middleware.IPAndEmailKey("auth-ip-email"), limiterStorage),
		AuthIPRateLimit:      middleware.NewRateLimiter(deps.Config.RateLimit.AuthIPPerMinute, time.Minute, middleware.IPKey("auth-ip"), limiterStorage),
		AuthEmailRateLimit: middleware.NewFailureRateLimiter(deps.Config.RateLimit.AuthEmailFailuresPerMinute, time.Minute,
			middleware.EmailKey("auth-email"), limiterStorage),
		RefreshRateLimit: middleware.NewRateLimiter(authLimit, time.Minute, middleware.IPKey("refresh"), limiterStorage),
	}
	routes.Setup()

	if deps.Config.Docs.Enabled {
		docs.Register(deps.App, api.OpenAPI)
	}
	return &WebComponents{Contacts: contactUseCase, Tokens: tokens, Metrics: deps.Metrics}
}
