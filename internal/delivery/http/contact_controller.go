package http

import (
	"github.com/primestrider/golang-clean-architecture/internal/delivery/http/middleware"
	"github.com/primestrider/golang-clean-architecture/internal/model"
	"github.com/primestrider/golang-clean-architecture/internal/usecase"
	"math"

	"github.com/gofiber/fiber/v3"
	"log/slog"
)

type ContactController struct {
	UseCase *usecase.ContactUseCase
	Log     *slog.Logger
}

func NewContactController(useCase *usecase.ContactUseCase, log *slog.Logger) *ContactController {
	return &ContactController{
		UseCase: useCase,
		Log:     log,
	}
}

func (c *ContactController) Create(ctx fiber.Ctx) error {
	auth := middleware.GetUser(ctx)

	request := new(model.CreateContactRequest)
	if err := ctx.Bind().Body(request); err != nil {
		c.Log.Error("error parsing request body", "error", err)
		return fiber.ErrBadRequest
	}
	request.UserId = auth.ID

	response, err := c.UseCase.Create(ctx.Context(), request)
	if err != nil {
		c.Log.Error("error creating contact", "error", err)
		return err
	}

	return ctx.JSON(model.WebResponse[*model.ContactResponse]{Data: response})
}

func (c *ContactController) List(ctx fiber.Ctx) error {
	auth := middleware.GetUser(ctx)

	request := &model.SearchContactRequest{
		UserId: auth.ID,
		Name:   ctx.Query("name", ""),
		Email:  ctx.Query("email", ""),
		Phone:  ctx.Query("phone", ""),
		Page:   fiber.Query[int](ctx, "page", 1),
		Size:   fiber.Query[int](ctx, "size", 10),
	}

	responses, total, err := c.UseCase.Search(ctx.Context(), request)
	if err != nil {
		c.Log.Error("error searching contact", "error", err)
		return err
	}

	paging := &model.PageMetadata{
		Page:      request.Page,
		Size:      request.Size,
		TotalItem: total,
		TotalPage: int64(math.Ceil(float64(total) / float64(request.Size))),
	}

	return ctx.JSON(model.WebResponse[[]model.ContactResponse]{
		Data:   responses,
		Paging: paging,
	})
}

func (c *ContactController) Get(ctx fiber.Ctx) error {
	auth := middleware.GetUser(ctx)

	request := &model.GetContactRequest{
		UserId: auth.ID,
		ID:     ctx.Params("contactId"),
	}

	response, err := c.UseCase.Get(ctx.Context(), request)
	if err != nil {
		c.Log.Error("error getting contact", "error", err)
		return err
	}

	return ctx.JSON(model.WebResponse[*model.ContactResponse]{Data: response})
}

func (c *ContactController) Update(ctx fiber.Ctx) error {
	auth := middleware.GetUser(ctx)

	request := new(model.UpdateContactRequest)
	if err := ctx.Bind().Body(request); err != nil {
		c.Log.Error("error parsing request body", "error", err)
		return fiber.ErrBadRequest
	}

	request.UserId = auth.ID
	request.ID = ctx.Params("contactId")

	response, err := c.UseCase.Update(ctx.Context(), request)
	if err != nil {
		c.Log.Error("error updating contact", "error", err)
		return err
	}

	return ctx.JSON(model.WebResponse[*model.ContactResponse]{Data: response})
}

func (c *ContactController) Delete(ctx fiber.Ctx) error {
	auth := middleware.GetUser(ctx)
	contactId := ctx.Params("contactId")

	request := &model.DeleteContactRequest{
		UserId: auth.ID,
		ID:     contactId,
	}

	if err := c.UseCase.Delete(ctx.Context(), request); err != nil {
		c.Log.Error("error deleting contact", "error", err)
		return err
	}

	return ctx.JSON(model.WebResponse[bool]{Data: true})
}
