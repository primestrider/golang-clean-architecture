package middleware

import (
	"github.com/gofiber/fiber/v3"
	"github.com/primestrider/golang-clean-architecture/internal/model"
	"github.com/primestrider/golang-clean-architecture/internal/observability"
	"github.com/primestrider/golang-clean-architecture/internal/usecase"
)

// authLocalsKey adalah kunci fiber.Locals untuk menyimpan identitas login.
const authLocalsKey = "auth"

// NewAuth membuat middleware yang mewajibkan token pada header
// Authorization. Jika token valid, identitas user disimpan di Locals dan id
// user dimasukkan ke context supaya ikut tercatat di log.
func NewAuth(users *usecase.UserUseCase) fiber.Handler {
	return func(c fiber.Ctx) error {
		req := &model.VerifyUserRequest{Token: c.Get(fiber.HeaderAuthorization)}
		auth, err := users.Verify(c.Context(), req)
		if err != nil {
			return err
		}
		c.Locals(authLocalsKey, auth)
		c.SetContext(observability.WithUserID(c.Context(), auth.ID))
		return c.Next()
	}
}

// GetUser mengambil identitas user yang login. Hanya boleh dipanggil di
// handler yang berada di belakang middleware NewAuth.
func GetUser(c fiber.Ctx) *model.Auth {
	auth, _ := c.Locals(authLocalsKey).(*model.Auth)
	return auth
}
