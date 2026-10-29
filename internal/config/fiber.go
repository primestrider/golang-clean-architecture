package config

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/spf13/viper"
)

func NewFiber(config *viper.Viper, log *slog.Logger) *fiber.App {
	var app = fiber.New(fiber.Config{
		AppName:      config.GetString("app.name"),
		ErrorHandler: NewErrorHandler(log),
	})

	// turn handler panics into 500 responses instead of crashing the server
	app.Use(recover.New())

	return app
}

func NewErrorHandler(log *slog.Logger) fiber.ErrorHandler {
	return func(ctx fiber.Ctx, err error) error {
		var e *fiber.Error
		if !errors.As(err, &e) {
			// unexpected errors (including recovered panics) must not leak details to the client
			log.Error("Unhandled error", "error", err, "method", ctx.Method(), "path", ctx.Path())
			e = fiber.ErrInternalServerError
		}

		return ctx.Status(e.Code).JSON(fiber.Map{
			"errors": e.Message,
		})
	}
}
