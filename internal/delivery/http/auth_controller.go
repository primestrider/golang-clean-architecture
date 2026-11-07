package http

import (
	"github.com/gofiber/fiber/v3"
	"github.com/primestrider/golang-clean-architecture/internal/model"
	"github.com/primestrider/golang-clean-architecture/internal/usecase"
)

// AuthController menangani endpoint sesi: login, refresh, dan logout.
type AuthController struct {
	auth *usecase.AuthUseCase
}

// NewAuthController membuat AuthController.
func NewAuthController(auth *usecase.AuthUseCase) *AuthController {
	return &AuthController{auth: auth}
}

// Login menangani POST /api/v1/auth/login.
func (h *AuthController) Login(c fiber.Ctx) error {
	req := new(model.LoginRequest)
	if err := c.Bind().Body(req); err != nil {
		return fiber.ErrBadRequest
	}
	req.UserAgent = c.Get(fiber.HeaderUserAgent)
	req.IP = c.IP()
	res, err := h.auth.Login(c.Context(), req)
	if err != nil {
		return err
	}
	return c.JSON(model.WebResponse[*model.TokenResponse]{Data: res})
}

// Refresh menangani POST /api/v1/auth/refresh.
func (h *AuthController) Refresh(c fiber.Ctx) error {
	req := new(model.RefreshRequest)
	if err := c.Bind().Body(req); err != nil {
		return fiber.ErrBadRequest
	}
	req.UserAgent = c.Get(fiber.HeaderUserAgent)
	req.IP = c.IP()
	res, err := h.auth.Refresh(c.Context(), req)
	if err != nil {
		return err
	}
	return c.JSON(model.WebResponse[*model.TokenResponse]{Data: res})
}

// Logout menangani POST /api/v1/auth/logout dan menjawab 204 No Content.
func (h *AuthController) Logout(c fiber.Ctx) error {
	req := new(model.LogoutRequest)
	if err := c.Bind().Body(req); err != nil {
		return fiber.ErrBadRequest
	}
	if err := h.auth.Logout(c.Context(), req); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
