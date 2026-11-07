package http

import (
	"github.com/gofiber/fiber/v3"
	"github.com/primestrider/golang-clean-architecture/internal/delivery/http/middleware"
	"github.com/primestrider/golang-clean-architecture/internal/model"
	"github.com/primestrider/golang-clean-architecture/internal/usecase"
)

// AddressController menangani endpoint alamat di bawah sebuah kontak.
type AddressController struct {
	addresses *usecase.AddressUseCase
}

// NewAddressController membuat AddressController.
func NewAddressController(addresses *usecase.AddressUseCase) *AddressController {
	return &AddressController{addresses: addresses}
}

// Create menangani POST /api/v1/contacts/:contactId/addresses.
func (h *AddressController) Create(c fiber.Ctx) error {
	req := new(model.CreateAddressRequest)
	if err := c.Bind().Body(req); err != nil {
		return fiber.ErrBadRequest
	}
	req.UserID = middleware.CurrentAuth(c).UserID
	req.ContactID = c.Params("contactId")
	res, err := h.addresses.Create(c.Context(), req)
	if err != nil {
		return err
	}
	return c.JSON(model.WebResponse[*model.AddressResponse]{Data: res})
}

// List menangani GET /api/v1/contacts/:contactId/addresses.
func (h *AddressController) List(c fiber.Ctx) error {
	req := &model.ListAddressRequest{UserID: middleware.CurrentAuth(c).UserID, ContactID: c.Params("contactId")}
	res, err := h.addresses.List(c.Context(), req)
	if err != nil {
		return err
	}
	return c.JSON(model.WebResponse[[]model.AddressResponse]{Data: res})
}

// Get menangani GET /api/v1/contacts/:contactId/addresses/:addressId.
func (h *AddressController) Get(c fiber.Ctx) error {
	req := &model.GetAddressRequest{
		UserID:    middleware.CurrentAuth(c).UserID,
		ContactID: c.Params("contactId"),
		ID:        c.Params("addressId"),
	}
	res, err := h.addresses.Get(c.Context(), req)
	if err != nil {
		return err
	}
	return c.JSON(model.WebResponse[*model.AddressResponse]{Data: res})
}

// Update menangani PUT /api/v1/contacts/:contactId/addresses/:addressId.
func (h *AddressController) Update(c fiber.Ctx) error {
	req := new(model.UpdateAddressRequest)
	if err := c.Bind().Body(req); err != nil {
		return fiber.ErrBadRequest
	}
	req.UserID = middleware.CurrentAuth(c).UserID
	req.ContactID = c.Params("contactId")
	req.ID = c.Params("addressId")
	res, err := h.addresses.Update(c.Context(), req)
	if err != nil {
		return err
	}
	return c.JSON(model.WebResponse[*model.AddressResponse]{Data: res})
}

// Delete menangani DELETE /api/v1/contacts/:contactId/addresses/:addressId.
func (h *AddressController) Delete(c fiber.Ctx) error {
	req := &model.DeleteAddressRequest{
		UserID:    middleware.CurrentAuth(c).UserID,
		ContactID: c.Params("contactId"),
		ID:        c.Params("addressId"),
	}
	if err := h.addresses.Delete(c.Context(), req); err != nil {
		return err
	}
	return c.JSON(model.WebResponse[bool]{Data: true})
}
