package route

import (
	"github.com/gofiber/fiber/v3"
	"github.com/primestrider/golang-clean-architecture/internal/delivery/http"
	"github.com/primestrider/golang-clean-architecture/internal/delivery/http/middleware"
	"github.com/primestrider/golang-clean-architecture/internal/entity"
)

// RouteConfig berisi semua controller dan middleware yang dibutuhkan untuk
// mendaftarkan route.
type RouteConfig struct {
	App               *fiber.App
	AuthController    *http.AuthController
	UserController    *http.UserController
	ContactController *http.ContactController
	AddressController *http.AddressController
	AdminController   *http.AdminController
	AuthMiddleware    fiber.Handler
	// Tiga limiter independen untuk login dan registrasi:
	//   - AuthIPEmailRateLimit: satu IP menebak password satu akun;
	//   - AuthIPRateLimit: satu IP mencoba banyak akun (credential stuffing);
	//   - AuthEmailRateLimit: banyak IP menebak satu akun (hanya kegagalan
	//     yang dihitung, supaya pemilik akun tidak mudah dikunci penyerang).
	AuthIPEmailRateLimit fiber.Handler
	AuthIPRateLimit      fiber.Handler
	AuthEmailRateLimit   fiber.Handler
	// RefreshRateLimit membatasi refresh token per IP.
	RefreshRateLimit fiber.Handler
}

// Setup mendaftarkan semua route di bawah prefix /api/v1. Versi di URL
// memungkinkan perubahan yang tidak kompatibel dirilis sebagai /api/v2
// tanpa memutus client lama.
func (c *RouteConfig) Setup() {
	api := c.App.Group("/api/v1")
	c.setupGuestRoutes(api)
	authed := api.Group("", c.AuthMiddleware)
	c.setupAuthRoutes(authed)
	c.setupAdminRoutes(authed.Group("/admin", middleware.RequireRole(entity.RoleAdmin)))
}

// setupAdminRoutes mendaftarkan endpoint yang hanya boleh diakses admin.
func (c *RouteConfig) setupAdminRoutes(admin fiber.Router) {
	admin.Get("/users", c.AdminController.ListUsers)
	admin.Patch("/users/:id/role", c.AdminController.ChangeRole)
}

// setupGuestRoutes mendaftarkan endpoint yang bisa diakses tanpa login.
func (c *RouteConfig) setupGuestRoutes(api fiber.Router) {
	// RenderErrors membuat limiter kegagalan per email bisa melihat status
	// error (401, 409, ...) yang dikembalikan controller.
	api.Post("/users", c.AuthIPRateLimit, c.AuthIPEmailRateLimit, c.AuthEmailRateLimit, middleware.RenderErrors(), c.UserController.Register)
	api.Post("/auth/login", c.AuthIPRateLimit, c.AuthIPEmailRateLimit, c.AuthEmailRateLimit, middleware.RenderErrors(), c.AuthController.Login)
	api.Post("/auth/refresh", c.RefreshRateLimit, c.AuthController.Refresh)
	api.Post("/auth/logout", c.AuthController.Logout)
}

// setupAuthRoutes mendaftarkan endpoint yang membutuhkan login.
func (c *RouteConfig) setupAuthRoutes(api fiber.Router) {
	api.Patch("/users/_current", c.UserController.Update)
	api.Get("/users/_current", c.UserController.Current)

	api.Get("/contacts", c.ContactController.List)
	api.Post("/contacts", c.ContactController.Create)
	api.Put("/contacts/:contactId", c.ContactController.Update)
	api.Get("/contacts/:contactId", c.ContactController.Get)
	api.Delete("/contacts/:contactId", c.ContactController.Delete)

	api.Get("/contacts/:contactId/addresses", c.AddressController.List)
	api.Post("/contacts/:contactId/addresses", c.AddressController.Create)
	api.Put("/contacts/:contactId/addresses/:addressId", c.AddressController.Update)
	api.Get("/contacts/:contactId/addresses/:addressId", c.AddressController.Get)
	api.Delete("/contacts/:contactId/addresses/:addressId", c.AddressController.Delete)
}
