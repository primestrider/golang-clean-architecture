package http

import (
	"golang-clean-architecture/internal/delivery/http/middleware"
	"golang-clean-architecture/internal/model"
	"golang-clean-architecture/internal/usecase"
	"log/slog"

	"github.com/gofiber/fiber/v3"
)

type UserController struct {
	Log     *slog.Logger
	UseCase *usecase.UserUseCase
}

func NewUserController(useCase *usecase.UserUseCase, logger *slog.Logger) *UserController {
	return &UserController{
		Log:     logger,
		UseCase: useCase,
	}
}

func (c *UserController) Register(ctx fiber.Ctx) error {
	request := new(model.RegisterUserRequest)
	err := ctx.Bind().Body(request)
	if err != nil {
		c.Log.Warn("Failed to parse request body", "error", err)
		return fiber.ErrBadRequest
	}

	response, err := c.UseCase.Create(ctx.Context(), request)
	if err != nil {
		c.Log.Warn("Failed to register user", "error", err)
		return err
	}

	return ctx.JSON(model.WebResponse[*model.UserResponse]{Data: response})
}

func (c *UserController) Login(ctx fiber.Ctx) error {
	request := new(model.LoginUserRequest)
	err := ctx.Bind().Body(request)
	if err != nil {
		c.Log.Warn("Failed to parse request body", "error", err)
		return fiber.ErrBadRequest
	}

	response, err := c.UseCase.Login(ctx.Context(), request)
	if err != nil {
		c.Log.Warn("Failed to login user", "error", err)
		return err
	}

	return ctx.JSON(model.WebResponse[*model.UserResponse]{Data: response})
}

func (c *UserController) Current(ctx fiber.Ctx) error {
	auth := middleware.GetUser(ctx)

	request := &model.GetUserRequest{
		ID: auth.ID,
	}

	response, err := c.UseCase.Current(ctx.Context(), request)
	if err != nil {
		c.Log.Warn("Failed to get current user", "error", err)
		return err
	}

	return ctx.JSON(model.WebResponse[*model.UserResponse]{Data: response})
}

func (c *UserController) Logout(ctx fiber.Ctx) error {
	auth := middleware.GetUser(ctx)

	request := &model.LogoutUserRequest{
		ID: auth.ID,
	}

	response, err := c.UseCase.Logout(ctx.Context(), request)
	if err != nil {
		c.Log.Warn("Failed to logout user", "error", err)
		return err
	}

	return ctx.JSON(model.WebResponse[bool]{Data: response})
}

func (c *UserController) Update(ctx fiber.Ctx) error {
	auth := middleware.GetUser(ctx)

	request := new(model.UpdateUserRequest)
	if err := ctx.Bind().Body(request); err != nil {
		c.Log.Warn("Failed to parse request body", "error", err)
		return fiber.ErrBadRequest
	}

	request.ID = auth.ID
	response, err := c.UseCase.Update(ctx.Context(), request)
	if err != nil {
		c.Log.Warn("Failed to update user", "error", err)
		return err
	}

	return ctx.JSON(model.WebResponse[*model.UserResponse]{Data: response})
}
