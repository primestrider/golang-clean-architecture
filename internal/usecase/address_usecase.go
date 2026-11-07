package usecase

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/primestrider/golang-clean-architecture/internal/entity"
	"github.com/primestrider/golang-clean-architecture/internal/model"
	"github.com/primestrider/golang-clean-architecture/internal/model/converter"
	"github.com/primestrider/golang-clean-architecture/internal/validation"
)

// AddressUseCase berisi aturan bisnis untuk alamat kontak.
//
// Setiap operasi lebih dulu memastikan kontak induknya milik user yang
// login, sehingga alamat milik user lain tidak bisa disentuh walaupun id
// alamatnya diketahui.
type AddressUseCase struct {
	log       *slog.Logger
	validate  *validation.Validator
	tx        Transactor
	contacts  ContactRepository
	addresses AddressRepository
	events    EventPublisher
	audit     *AuditRecorder
}

// NewAddressUseCase membuat AddressUseCase dengan dependency-nya.
func NewAddressUseCase(log *slog.Logger, validate *validation.Validator, tx Transactor,
	contacts ContactRepository, addresses AddressRepository, events EventPublisher, audit *AuditRecorder) *AddressUseCase {
	return &AddressUseCase{log: log, validate: validate, tx: tx, contacts: contacts, addresses: addresses, events: events, audit: audit}
}

// Create menambahkan alamat ke kontak milik user yang login.
func (c *AddressUseCase) Create(ctx context.Context, req *model.CreateAddressRequest) (*model.AddressResponse, error) {
	if err := c.validate.Struct(req); err != nil {
		return nil, err
	}

	address := &entity.Address{
		ID:         uuid.NewString(),
		ContactID:  req.ContactID,
		Street:     req.Street,
		City:       req.City,
		Province:   req.Province,
		PostalCode: req.PostalCode,
		Country:    req.Country,
	}

	err := c.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		if _, err := c.contacts.FindByIDAndUserID(ctx, req.ContactID, req.UserID); err != nil {
			return err
		}
		if err := c.addresses.Create(ctx, address); err != nil {
			return err
		}
		if err := c.audit.Record(ctx, "address.created", "address", address.ID, nil, converter.AddressToResponse(address)); err != nil {
			return err
		}
		return c.events.Publish(ctx, converter.AddressToEvent(address))
	})
	if err != nil {
		return nil, err
	}
	return converter.AddressToResponse(address), nil
}

// Update mengubah alamat milik kontak user yang login.
func (c *AddressUseCase) Update(ctx context.Context, req *model.UpdateAddressRequest) (*model.AddressResponse, error) {
	if err := c.validate.Struct(req); err != nil {
		return nil, err
	}

	var address *entity.Address
	err := c.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		if _, err := c.contacts.FindByIDAndUserID(ctx, req.ContactID, req.UserID); err != nil {
			return err
		}
		var err error
		address, err = c.addresses.FindByIDAndContactID(ctx, req.ID, req.ContactID)
		if err != nil {
			return err
		}
		before := converter.AddressToResponse(address)
		address.Street = req.Street
		address.City = req.City
		address.Province = req.Province
		address.PostalCode = req.PostalCode
		address.Country = req.Country
		if err := c.addresses.UpdateWithVersion(ctx, address, req.Version); err != nil {
			return err
		}
		if err := c.audit.Record(ctx, "address.updated", "address", address.ID, before, converter.AddressToResponse(address)); err != nil {
			return err
		}
		return c.events.Publish(ctx, converter.AddressToEvent(address))
	})
	if err != nil {
		return nil, err
	}
	return converter.AddressToResponse(address), nil
}

// Get mengambil satu alamat.
func (c *AddressUseCase) Get(ctx context.Context, req *model.GetAddressRequest) (*model.AddressResponse, error) {
	if err := c.validate.Struct(req); err != nil {
		return nil, err
	}
	if _, err := c.contacts.FindByIDAndUserID(ctx, req.ContactID, req.UserID); err != nil {
		return nil, err
	}
	address, err := c.addresses.FindByIDAndContactID(ctx, req.ID, req.ContactID)
	if err != nil {
		return nil, err
	}
	return converter.AddressToResponse(address), nil
}

// Delete menghapus alamat.
func (c *AddressUseCase) Delete(ctx context.Context, req *model.DeleteAddressRequest) error {
	if err := c.validate.Struct(req); err != nil {
		return err
	}
	return c.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		if _, err := c.contacts.FindByIDAndUserID(ctx, req.ContactID, req.UserID); err != nil {
			return err
		}
		address, err := c.addresses.FindByIDAndContactID(ctx, req.ID, req.ContactID)
		if err != nil {
			return err
		}
		if err := c.addresses.Delete(ctx, address); err != nil {
			return err
		}
		return c.audit.Record(ctx, "address.deleted", "address", address.ID, converter.AddressToResponse(address), nil)
	})
}

// List mengembalikan semua alamat milik sebuah kontak.
func (c *AddressUseCase) List(ctx context.Context, req *model.ListAddressRequest) ([]model.AddressResponse, error) {
	if err := c.validate.Struct(req); err != nil {
		return nil, err
	}
	if _, err := c.contacts.FindByIDAndUserID(ctx, req.ContactID, req.UserID); err != nil {
		return nil, err
	}
	addresses, err := c.addresses.FindAllByContactID(ctx, req.ContactID)
	if err != nil {
		return nil, err
	}

	responses := make([]model.AddressResponse, len(addresses))
	for i := range addresses {
		responses[i] = *converter.AddressToResponse(&addresses[i])
	}
	return responses, nil
}
