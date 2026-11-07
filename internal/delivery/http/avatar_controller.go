package http

import (
	"github.com/gofiber/fiber/v3"
	"github.com/primestrider/golang-clean-architecture/internal/apperror"
	"github.com/primestrider/golang-clean-architecture/internal/delivery/http/middleware"
	"github.com/primestrider/golang-clean-architecture/internal/model"
	"github.com/primestrider/golang-clean-architecture/internal/usecase"
)

// AvatarController menangani upload dan penghapusan foto profil. Route-nya
// hanya didaftarkan bila object storage diaktifkan.
type AvatarController struct {
	avatars *usecase.AvatarUseCase
}

// NewAvatarController membuat AvatarController.
func NewAvatarController(avatars *usecase.AvatarUseCase) *AvatarController {
	return &AvatarController{avatars: avatars}
}

// Upload menangani PUT /api/v1/users/_current/avatar (multipart, field "file").
func (h *AvatarController) Upload(c fiber.Ctx) error {
	header, err := c.FormFile("file")
	if err != nil {
		return apperror.Validation([]apperror.FieldError{{Field: "file", Message: "is required"}})
	}
	file, err := header.Open()
	if err != nil {
		return apperror.Internal(err)
	}
	defer file.Close()

	res, err := h.avatars.Upload(c.Context(), middleware.CurrentAuth(c).UserID, file, header.Size)
	if err != nil {
		return err
	}
	return c.JSON(model.WebResponse[*model.UserResponse]{Data: res})
}

// Remove menangani DELETE /api/v1/users/_current/avatar.
func (h *AvatarController) Remove(c fiber.Ctx) error {
	if err := h.avatars.Remove(c.Context(), middleware.CurrentAuth(c).UserID); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
