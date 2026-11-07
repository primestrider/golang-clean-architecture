package usecase

import (
	"context"
	"github.com/primestrider/golang-clean-architecture/internal/entity"
	"github.com/primestrider/golang-clean-architecture/internal/gateway/messaging"
	"github.com/primestrider/golang-clean-architecture/internal/model"
	"github.com/primestrider/golang-clean-architecture/internal/model/converter"
	"github.com/primestrider/golang-clean-architecture/internal/repository"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/primestrider/golang-clean-architecture/internal/validation"
	"gorm.io/gorm"
	"log/slog"
)

type AddressUseCase struct {
	DB                *gorm.DB
	Log               *slog.Logger
	Validate          *validation.Validator
	AddressRepository *repository.AddressRepository
	ContactRepository *repository.ContactRepository
	AddressProducer   *messaging.AddressProducer
}

func NewAddressUseCase(db *gorm.DB, logger *slog.Logger, validate *validation.Validator,
	contactRepository *repository.ContactRepository, addressRepository *repository.AddressRepository,
	addressProducer *messaging.AddressProducer) *AddressUseCase {
	return &AddressUseCase{
		DB:                db,
		Log:               logger,
		Validate:          validate,
		ContactRepository: contactRepository,
		AddressRepository: addressRepository,
		AddressProducer:   addressProducer,
	}
}

func (c *AddressUseCase) Create(ctx context.Context, request *model.CreateAddressRequest) (*model.AddressResponse, error) {
	tx := c.DB.WithContext(ctx).Begin()
	defer tx.Rollback()

	if err := c.Validate.Struct(request); err != nil {
		c.Log.Error("failed to validate request body", "error", err)
		return nil, fiber.ErrBadRequest
	}

	contact := new(entity.Contact)
	if err := c.ContactRepository.FindByIdAndUserId(tx, contact, request.ContactId, request.UserId); err != nil {
		c.Log.Error("failed to find contact", "error", err)
		return nil, fiber.ErrNotFound
	}

	address := &entity.Address{
		ID:         uuid.NewString(),
		ContactId:  contact.ID,
		Street:     request.Street,
		City:       request.City,
		Province:   request.Province,
		PostalCode: request.PostalCode,
		Country:    request.Country,
	}

	if err := c.AddressRepository.Create(tx, address); err != nil {
		c.Log.Error("failed to create address", "error", err)
		return nil, fiber.ErrInternalServerError
	}

	if err := tx.Commit().Error; err != nil {
		c.Log.Error("failed to commit transaction", "error", err)
		return nil, fiber.ErrInternalServerError
	}

	if c.AddressProducer != nil {
		event := converter.AddressToEvent(address)
		if err := c.AddressProducer.Send(event); err != nil {
			c.Log.Error("failed to publish address created event", "error", err)
			return nil, fiber.ErrInternalServerError
		}
		c.Log.Info("Published address created event")
	} else {
		c.Log.Info("Kafka producer is disabled, skipping address created event")
	}

	return converter.AddressToResponse(address), nil
}

func (c *AddressUseCase) Update(ctx context.Context, request *model.UpdateAddressRequest) (*model.AddressResponse, error) {
	tx := c.DB.WithContext(ctx).Begin()
	defer tx.Rollback()

	if err := c.Validate.Struct(request); err != nil {
		c.Log.Error("failed to validate request body", "error", err)
		return nil, fiber.ErrBadRequest
	}

	contact := new(entity.Contact)
	if err := c.ContactRepository.FindByIdAndUserId(tx, contact, request.ContactId, request.UserId); err != nil {
		c.Log.Error("failed to find contact", "error", err)
		return nil, fiber.ErrNotFound
	}

	address := new(entity.Address)
	if err := c.AddressRepository.FindByIdAndContactId(tx, address, request.ID, contact.ID); err != nil {
		c.Log.Error("failed to find address", "error", err)
		return nil, fiber.ErrNotFound
	}

	address.Street = request.Street
	address.City = request.City
	address.Province = request.Province
	address.PostalCode = request.PostalCode
	address.Country = request.Country

	if err := c.AddressRepository.Update(tx, address); err != nil {
		c.Log.Error("failed to update address", "error", err)
		return nil, fiber.ErrInternalServerError
	}

	if err := tx.Commit().Error; err != nil {
		c.Log.Error("failed to commit transaction", "error", err)
		return nil, fiber.ErrInternalServerError
	}

	if c.AddressProducer != nil {
		event := converter.AddressToEvent(address)
		if err := c.AddressProducer.Send(event); err != nil {
			c.Log.Error("failed to publish address updated event", "error", err)
			return nil, fiber.ErrInternalServerError
		}
		c.Log.Info("Published address updated event")
	} else {
		c.Log.Info("Kafka producer is disabled, skipping address updated event")
	}

	return converter.AddressToResponse(address), nil
}

func (c *AddressUseCase) Get(ctx context.Context, request *model.GetAddressRequest) (*model.AddressResponse, error) {
	tx := c.DB.WithContext(ctx).Begin()
	defer tx.Rollback()

	contact := new(entity.Contact)
	if err := c.ContactRepository.FindByIdAndUserId(tx, contact, request.ContactId, request.UserId); err != nil {
		c.Log.Error("failed to find contact", "error", err)
		return nil, fiber.ErrNotFound
	}

	address := new(entity.Address)
	if err := c.AddressRepository.FindByIdAndContactId(tx, address, request.ID, request.ContactId); err != nil {
		c.Log.Error("failed to find address", "error", err)
		return nil, fiber.ErrNotFound
	}

	if err := tx.Commit().Error; err != nil {
		c.Log.Error("failed to commit transaction", "error", err)
		return nil, fiber.ErrInternalServerError
	}

	return converter.AddressToResponse(address), nil
}

func (c *AddressUseCase) Delete(ctx context.Context, request *model.DeleteAddressRequest) error {
	tx := c.DB.WithContext(ctx).Begin()
	defer tx.Rollback()

	contact := new(entity.Contact)
	if err := c.ContactRepository.FindByIdAndUserId(tx, contact, request.ContactId, request.UserId); err != nil {
		c.Log.Error("failed to find contact", "error", err)
		return fiber.ErrNotFound
	}

	address := new(entity.Address)
	if err := c.AddressRepository.FindByIdAndContactId(tx, address, request.ID, request.ContactId); err != nil {
		c.Log.Error("failed to find address", "error", err)
		return fiber.ErrNotFound
	}

	if err := c.AddressRepository.Delete(tx, address); err != nil {
		c.Log.Error("failed to delete address", "error", err)
		return fiber.ErrInternalServerError
	}

	if err := tx.Commit().Error; err != nil {
		c.Log.Error("failed to commit transaction", "error", err)
		return fiber.ErrInternalServerError
	}

	return nil
}

func (c *AddressUseCase) List(ctx context.Context, request *model.ListAddressRequest) ([]model.AddressResponse, error) {
	tx := c.DB.WithContext(ctx).Begin()
	defer tx.Rollback()

	contact := new(entity.Contact)
	if err := c.ContactRepository.FindByIdAndUserId(tx, contact, request.ContactId, request.UserId); err != nil {
		c.Log.Error("failed to find contact", "error", err)
		return nil, fiber.ErrNotFound
	}

	addresses, err := c.AddressRepository.FindAllByContactId(tx, contact.ID)
	if err != nil {
		c.Log.Error("failed to find addresses", "error", err)
		return nil, fiber.ErrInternalServerError
	}

	if err := tx.Commit().Error; err != nil {
		c.Log.Error("failed to commit transaction", "error", err)
		return nil, fiber.ErrInternalServerError
	}

	responses := make([]model.AddressResponse, len(addresses))
	for i, address := range addresses {
		responses[i] = *converter.AddressToResponse(&address)
	}

	return responses, nil
}
