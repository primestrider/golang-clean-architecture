package route

import (
	"github.com/gofiber/fiber/v3"
	"github.com/primestrider/golang-clean-architecture/internal/delivery/http"
)

// RouteConfig berisi semua controller dan middleware yang dibutuhkan untuk
// mendaftarkan route.
type RouteConfig struct {
	App               *fiber.App
	AuthController    *http.AuthController
	UserController    *http.UserController
	ContactController *http.ContactController
	AddressController *http.AddressController
	AuthMiddleware    fiber.Handler
	// AuthRateLimit membatasi login dan registrasi per IP+email.
	AuthRateLimit fiber.Handler
	// RefreshRateLimit membatasi refresh token per IP.
	RefreshRateLimit fiber.Handler
}

// Setup mendaftarkan semua route di bawah prefix /api/v1. Versi di URL
// memungkinkan perubahan yang tidak kompatibel dirilis sebagai /api/v2
// tanpa memutus client lama.
func (c *RouteConfig) Setup() {
	api := c.App.Group("/api/v1")
	c.setupGuestRoutes(api)
	c.setupAuthRoutes(api.Group("", c.AuthMiddleware))
}

// setupGuestRoutes mendaftarkan endpoint yang bisa diakses tanpa login.
func (c *RouteConfig) setupGuestRoutes(api fiber.Router) {
	api.Post("/users", c.AuthRateLimit, c.UserController.Register)
	api.Post("/auth/login", c.AuthRateLimit, c.AuthController.Login)
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
