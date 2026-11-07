package http

import (
	"github.com/gofiber/fiber/v3"
	"github.com/primestrider/golang-clean-architecture/internal/delivery/http/middleware"
	"github.com/primestrider/golang-clean-architecture/internal/model"
	"github.com/primestrider/golang-clean-architecture/internal/usecase"
)

// UserController menangani endpoint akun user.
//
// Pola setiap handler sama: parse body, isi field dari token login, panggil
// use case, lalu bungkus hasil dalam WebResponse. Error cukup dikembalikan;
// error handler global yang mengubahnya menjadi Problem Details dan
// mencatatnya ke log.
type UserController struct {
	users *usecase.UserUseCase
}

// NewUserController membuat UserController.
func NewUserController(users *usecase.UserUseCase) *UserController {
	return &UserController{users: users}
}

// Register menangani POST /api/v1/users.
func (h *UserController) Register(c fiber.Ctx) error {
	req := new(model.RegisterUserRequest)
	if err := c.Bind().Body(req); err != nil {
		return fiber.ErrBadRequest
	}
	res, err := h.users.Create(c.Context(), req)
	if err != nil {
		return err
	}
	return c.JSON(model.WebResponse[*model.UserResponse]{Data: res})
}

// Login menangani POST /api/v1/users/_login.
func (h *UserController) Login(c fiber.Ctx) error {
	req := new(model.LoginUserRequest)
	if err := c.Bind().Body(req); err != nil {
		return fiber.ErrBadRequest
	}
	res, err := h.users.Login(c.Context(), req)
	if err != nil {
		return err
	}
	return c.JSON(model.WebResponse[*model.UserResponse]{Data: res})
}

// Current menangani GET /api/v1/users/_current.
func (h *UserController) Current(c fiber.Ctx) error {
	res, err := h.users.Current(c.Context(), &model.GetUserRequest{ID: middleware.GetUser(c).ID})
	if err != nil {
		return err
	}
	return c.JSON(model.WebResponse[*model.UserResponse]{Data: res})
}

// Logout menangani DELETE /api/v1/users.
func (h *UserController) Logout(c fiber.Ctx) error {
	res, err := h.users.Logout(c.Context(), &model.LogoutUserRequest{ID: middleware.GetUser(c).ID})
	if err != nil {
		return err
	}
	return c.JSON(model.WebResponse[bool]{Data: res})
}

// Update menangani PATCH /api/v1/users/_current.
func (h *UserController) Update(c fiber.Ctx) error {
	req := new(model.UpdateUserRequest)
	if err := c.Bind().Body(req); err != nil {
		return fiber.ErrBadRequest
	}
	req.ID = middleware.GetUser(c).ID
	res, err := h.users.Update(c.Context(), req)
	if err != nil {
		return err
	}
	return c.JSON(model.WebResponse[*model.UserResponse]{Data: res})
}
