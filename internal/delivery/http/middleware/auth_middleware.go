package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/primestrider/golang-clean-architecture/internal/apperror"
	"github.com/primestrider/golang-clean-architecture/internal/model"
	"github.com/primestrider/golang-clean-architecture/internal/observability"
	"github.com/primestrider/golang-clean-architecture/internal/security"
)

// authLocalsKey adalah kunci fiber.Locals untuk menyimpan identitas login.
const authLocalsKey = "auth"

// TokenParser memverifikasi access token. Diimplementasikan oleh
// security.TokenIssuer.
type TokenParser interface {
	Parse(token string) (*security.Claims, error)
}

// NewAuth membuat middleware yang mewajibkan header
// "Authorization: Bearer <access token>".
//
// Token diverifikasi tanpa ke database (JWT membawa id dan role user yang
// sudah ditandatangani). Identitas user disimpan di Locals, dan id user
// dimasukkan ke context supaya ikut tercatat di setiap log request ini.
func NewAuth(tokens TokenParser) fiber.Handler {
	return func(c fiber.Ctx) error {
		scheme, token, ok := strings.Cut(c.Get(fiber.HeaderAuthorization), " ")
		if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" {
			return apperror.Unauthorized("missing bearer token")
		}
		claims, err := tokens.Parse(token)
		if err != nil {
			return err
		}
		c.Locals(authLocalsKey, &model.Auth{UserID: claims.UserID, Role: claims.Role})
		c.SetContext(observability.WithUserID(c.Context(), claims.UserID))
		return c.Next()
	}
}

// CurrentAuth mengambil identitas user yang login. Hanya boleh dipanggil di
// handler yang berada di belakang middleware NewAuth.
func CurrentAuth(c fiber.Ctx) *model.Auth {
	auth, _ := c.Locals(authLocalsKey).(*model.Auth)
	return auth
}
