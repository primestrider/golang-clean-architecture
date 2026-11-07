package middleware

import (
	"slices"

	"github.com/gofiber/fiber/v3"
	"github.com/primestrider/golang-clean-architecture/internal/apperror"
)

// RequireRole hanya meloloskan user yang role-nya termasuk roles. Harus
// dipasang setelah NewAuth. User tanpa role yang sesuai mendapat 403.
func RequireRole(roles ...string) fiber.Handler {
	return func(c fiber.Ctx) error {
		auth := CurrentAuth(c)
		if auth == nil || !slices.Contains(roles, auth.Role) {
			return apperror.Forbidden("insufficient permissions")
		}
		return c.Next()
	}
}
