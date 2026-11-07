package middleware

import (
	"github.com/primestrider/golang-clean-architecture/internal/model"
	"github.com/primestrider/golang-clean-architecture/internal/usecase"

	"github.com/gofiber/fiber/v3"
)

func NewAuth(userUserCase *usecase.UserUseCase) fiber.Handler {
	return func(ctx fiber.Ctx) error {
		request := &model.VerifyUserRequest{Token: ctx.Get("Authorization", "NOT_FOUND")}

		auth, err := userUserCase.Verify(ctx.Context(), request)
		if err != nil {
			userUserCase.Log.Warn("Failed find user by token", "error", err)
			return fiber.ErrUnauthorized
		}

		userUserCase.Log.Debug("authenticated user", "user_id", auth.ID)
		ctx.Locals("auth", auth)
		return ctx.Next()
	}
}

func GetUser(ctx fiber.Ctx) *model.Auth {
	return ctx.Locals("auth").(*model.Auth)
}
