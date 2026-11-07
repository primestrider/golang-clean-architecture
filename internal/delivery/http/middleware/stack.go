package middleware

import (
	"context"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/helmet"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/google/uuid"
	"github.com/primestrider/golang-clean-architecture/internal/apperror"
	"github.com/primestrider/golang-clean-architecture/internal/observability"
)

// HeaderRequestID adalah header untuk korelasi request antar layanan.
const HeaderRequestID = "X-Request-ID"

// validRequestID membatasi request id kiriman client: hanya huruf, angka,
// dan tanda minus, maksimal 64 karakter. Nilai lain diganti supaya client
// tidak bisa menyisipkan teks aneh ke log.
var validRequestID = regexp.MustCompile(`^[A-Za-z0-9-]{1,64}$`)

// StackConfig adalah pengaturan stack middleware global.
type StackConfig struct {
	CORSOrigins     []string
	RequestTimeout  time.Duration
	GlobalPerMinute int
	// Storage untuk rate limiter; nil berarti memori proses ini.
	Storage fiber.Storage
	// JSONBodyLimit adalah ukuran maksimum body JSON (byte). Batas server
	// bisa lebih besar untuk mengizinkan upload file.
	JSONBodyLimit int
	// Extra disisipkan setelah request id dan sebelum access log, misalnya
	// middleware tracing dan metrics.
	Extra []fiber.Handler
}

// Apply memasang middleware global dengan urutan yang disengaja:
//
//  1. recover: panic diubah menjadi 500, aplikasi tetap hidup;
//  2. request id: setiap request punya id untuk korelasi log;
//  3. extra (tracing, metrics);
//  4. access log: satu baris log per request;
//  5. helmet: header keamanan browser;
//  6. CORS: hanya origin yang diizinkan;
//  7. rate limit global per IP;
//  8. timeout: context request dibatalkan setelah batas waktu, sehingga
//     query database ikut berhenti;
//  9. tipe konten: body POST/PUT/PATCH harus JSON (atau multipart untuk upload).
func Apply(app *fiber.App, cfg StackConfig, log *slog.Logger) {
	app.Use(recover.New(recover.Config{
		EnableStackTrace: true,
		StackTraceHandler: func(c fiber.Ctx, e any) {
			log.ErrorContext(c.Context(), "panic recovered", "panic", e, "path", c.Path())
		},
	}))
	app.Use(requestID())
	for _, h := range cfg.Extra {
		app.Use(h)
	}
	app.Use(accessLog(log))
	app.Use(helmet.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins:  cfg.CORSOrigins,
		AllowHeaders:  []string{fiber.HeaderAuthorization, fiber.HeaderContentType, HeaderRequestID},
		ExposeHeaders: []string{HeaderRequestID, fiber.HeaderRetryAfter},
	}))
	app.Use(NewRateLimiter(cfg.GlobalPerMinute, time.Minute, func(c fiber.Ctx) string { return "global:" + c.IP() }, cfg.Storage))
	app.Use(timeout(cfg.RequestTimeout))
	app.Use(requireJSONBody(cfg.JSONBodyLimit))
}

// requestID memakai X-Request-ID dari client bila valid, atau membuat UUID
// baru. Id dikembalikan di header respons dan disimpan di context.
func requestID() fiber.Handler {
	return func(c fiber.Ctx) error {
		id := c.Get(HeaderRequestID)
		if !validRequestID.MatchString(id) {
			id = uuid.NewString()
		}
		c.Set(HeaderRequestID, id)
		c.SetContext(observability.WithRequestID(c.Context(), id))
		return c.Next()
	}
}

// accessLog mencatat satu baris per request setelah request selesai.
//
// Error dari handler langsung diserahkan ke error handler aplikasi di sini,
// supaya status yang dicatat adalah status yang benar-benar dikirim ke client.
func accessLog(log *slog.Logger) fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()
		if err := c.Next(); err != nil {
			if handlerErr := c.App().ErrorHandler(c, err); handlerErr != nil {
				return handlerErr
			}
		}
		log.InfoContext(c.Context(), "http request",
			"method", c.Method(),
			"path", c.Path(),
			"status", c.Response().StatusCode(),
			"duration_ms", time.Since(start).Milliseconds(),
			"ip", c.IP(),
		)
		return nil
	}
}

// timeout memberi batas waktu pada context request. Handler dan query
// database yang memakai context ini akan dibatalkan bila melewati batas.
func timeout(d time.Duration) fiber.Handler {
	return func(c fiber.Ctx) error {
		if d <= 0 {
			return c.Next()
		}
		ctx, cancel := context.WithTimeout(c.Context(), d)
		defer cancel()
		c.SetContext(ctx)
		return c.Next()
	}
}

// requireJSONBody menolak request berbody yang bukan JSON (atau multipart
// untuk upload file) dengan 415, alih-alih mencoba mem-parse format lain,
// dan menolak body JSON yang melebihi jsonLimit dengan 413.
func requireJSONBody(jsonLimit int) fiber.Handler {
	return func(c fiber.Ctx) error {
		switch c.Method() {
		case fiber.MethodPost, fiber.MethodPut, fiber.MethodPatch:
		default:
			return c.Next()
		}
		if len(c.Body()) == 0 {
			return c.Next()
		}
		ctype := strings.ToLower(c.Get(fiber.HeaderContentType))
		if strings.HasPrefix(ctype, fiber.MIMEMultipartForm) {
			return c.Next()
		}
		if strings.HasPrefix(ctype, fiber.MIMEApplicationJSON) {
			if jsonLimit > 0 && len(c.Body()) > jsonLimit {
				return apperror.New(apperror.CodePayloadTooLarge, "request body too large")
			}
			return c.Next()
		}
		return apperror.New(apperror.CodeUnsupportedMedia, "content type must be application/json")
	}
}
