package http

import (
	"github.com/gofiber/fiber/v3"
	"github.com/primestrider/golang-clean-architecture/internal/delivery/http/middleware"
	"github.com/primestrider/golang-clean-architecture/internal/model"
	"github.com/primestrider/golang-clean-architecture/internal/usecase"
)

// AdminController menangani endpoint khusus admin. Route-nya dilindungi
// middleware RequireRole("admin").
type AdminController struct {
	admin *usecase.AdminUseCase
}

// NewAdminController membuat AdminController.
func NewAdminController(admin *usecase.AdminUseCase) *AdminController {
	return &AdminController{admin: admin}
}

// ListUsers menangani GET /api/v1/admin/users?page=&size=.
func (h *AdminController) ListUsers(c fiber.Ctx) error {
	req := &model.ListUsersRequest{Page: fiber.Query[int](c, "page", 1), Size: fiber.Query[int](c, "size", 10)}
	res, paging, err := h.admin.ListUsers(c.Context(), req)
	if err != nil {
		return err
	}
	return c.JSON(model.WebResponse[[]model.UserResponse]{Data: res, Paging: paging})
}

// ChangeRole menangani PATCH /api/v1/admin/users/:id/role.
func (h *AdminController) ChangeRole(c fiber.Ctx) error {
	req := new(model.ChangeRoleRequest)
	if err := c.Bind().Body(req); err != nil {
		return fiber.ErrBadRequest
	}
	req.ActorID = middleware.CurrentAuth(c).UserID
	req.UserID = c.Params("id")
	res, err := h.admin.ChangeRole(c.Context(), req)
	if err != nil {
		return err
	}
	return c.JSON(model.WebResponse[*model.UserResponse]{Data: res})
}
