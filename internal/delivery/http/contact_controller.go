package http

import (
	"github.com/gofiber/fiber/v3"
	"github.com/primestrider/golang-clean-architecture/internal/delivery/http/middleware"
	"github.com/primestrider/golang-clean-architecture/internal/model"
	"github.com/primestrider/golang-clean-architecture/internal/usecase"
)

// ContactController menangani endpoint kontak.
type ContactController struct {
	contacts *usecase.ContactUseCase
}

// NewContactController membuat ContactController.
func NewContactController(contacts *usecase.ContactUseCase) *ContactController {
	return &ContactController{contacts: contacts}
}

// Create menangani POST /api/v1/contacts.
func (h *ContactController) Create(c fiber.Ctx) error {
	req := new(model.CreateContactRequest)
	if err := c.Bind().Body(req); err != nil {
		return fiber.ErrBadRequest
	}
	req.UserID = middleware.CurrentAuth(c).UserID
	res, err := h.contacts.Create(c.Context(), req)
	if err != nil {
		return err
	}
	return c.JSON(model.WebResponse[*model.ContactResponse]{Data: res})
}

// List menangani GET /api/v1/contacts dengan query name, email, phone,
// page (default 1), dan size (default 10).
func (h *ContactController) List(c fiber.Ctx) error {
	req := &model.SearchContactRequest{
		UserID: middleware.CurrentAuth(c).UserID,
		Name:   c.Query("name"),
		Email:  c.Query("email"),
		Phone:  c.Query("phone"),
		Page:   fiber.Query[int](c, "page", 1),
		Size:   fiber.Query[int](c, "size", 10),
	}
	res, paging, err := h.contacts.Search(c.Context(), req)
	if err != nil {
		return err
	}
	return c.JSON(model.WebResponse[[]model.ContactResponse]{Data: res, Paging: paging})
}

// Get menangani GET /api/v1/contacts/:contactId.
func (h *ContactController) Get(c fiber.Ctx) error {
	req := &model.GetContactRequest{UserID: middleware.CurrentAuth(c).UserID, ID: c.Params("contactId")}
	res, err := h.contacts.Get(c.Context(), req)
	if err != nil {
		return err
	}
	return c.JSON(model.WebResponse[*model.ContactResponse]{Data: res})
}

// Update menangani PUT /api/v1/contacts/:contactId.
func (h *ContactController) Update(c fiber.Ctx) error {
	req := new(model.UpdateContactRequest)
	if err := c.Bind().Body(req); err != nil {
		return fiber.ErrBadRequest
	}
	req.UserID = middleware.CurrentAuth(c).UserID
	req.ID = c.Params("contactId")
	res, err := h.contacts.Update(c.Context(), req)
	if err != nil {
		return err
	}
	return c.JSON(model.WebResponse[*model.ContactResponse]{Data: res})
}

// Delete menangani DELETE /api/v1/contacts/:contactId.
func (h *ContactController) Delete(c fiber.Ctx) error {
	req := &model.DeleteContactRequest{UserID: middleware.CurrentAuth(c).UserID, ID: c.Params("contactId")}
	if err := h.contacts.Delete(c.Context(), req); err != nil {
		return err
	}
	return c.JSON(model.WebResponse[bool]{Data: true})
}
