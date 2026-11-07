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

// ContactUseCase berisi aturan bisnis untuk kontak milik user.
type ContactUseCase struct {
	log       *slog.Logger
	validate  *validation.Validator
	tx        Transactor
	contacts  ContactRepository
	addresses AddressRepository
	events    EventPublisher
}

// NewContactUseCase membuat ContactUseCase dengan dependency-nya.
func NewContactUseCase(log *slog.Logger, validate *validation.Validator, tx Transactor,
	contacts ContactRepository, addresses AddressRepository, events EventPublisher) *ContactUseCase {
	return &ContactUseCase{log: log, validate: validate, tx: tx, contacts: contacts, addresses: addresses, events: events}
}

// Create membuat kontak baru untuk user yang login. Penyimpanan kontak dan
// penerbitan event terjadi dalam satu transaksi: keduanya berhasil, atau
// keduanya batal.
func (c *ContactUseCase) Create(ctx context.Context, req *model.CreateContactRequest) (*model.ContactResponse, error) {
	if err := c.validate.Struct(req); err != nil {
		return nil, err
	}

	contact := &entity.Contact{
		ID:        uuid.NewString(),
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Email:     req.Email,
		Phone:     req.Phone,
		UserID:    req.UserID,
	}

	err := c.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := c.contacts.Create(ctx, contact); err != nil {
			return err
		}
		return c.events.Publish(ctx, converter.ContactToEvent(contact))
	})
	if err != nil {
		return nil, err
	}
	return converter.ContactToResponse(contact), nil
}

// Update mengubah kontak milik user yang login.
func (c *ContactUseCase) Update(ctx context.Context, req *model.UpdateContactRequest) (*model.ContactResponse, error) {
	if err := c.validate.Struct(req); err != nil {
		return nil, err
	}

	var contact *entity.Contact
	err := c.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		var err error
		contact, err = c.contacts.FindByIDAndUserID(ctx, req.ID, req.UserID)
		if err != nil {
			return err
		}
		contact.FirstName = req.FirstName
		contact.LastName = req.LastName
		contact.Email = req.Email
		contact.Phone = req.Phone
		if err := c.contacts.UpdateWithVersion(ctx, contact, req.Version); err != nil {
			return err
		}
		return c.events.Publish(ctx, converter.ContactToEvent(contact))
	})
	if err != nil {
		return nil, err
	}
	return converter.ContactToResponse(contact), nil
}

// Get mengambil satu kontak milik user yang login. Operasi baca tidak
// membuka transaksi.
func (c *ContactUseCase) Get(ctx context.Context, req *model.GetContactRequest) (*model.ContactResponse, error) {
	if err := c.validate.Struct(req); err != nil {
		return nil, err
	}
	contact, err := c.contacts.FindByIDAndUserID(ctx, req.ID, req.UserID)
	if err != nil {
		return nil, err
	}
	return converter.ContactToResponse(contact), nil
}

// Delete men-soft-delete kontak milik user yang login beserta semua
// alamatnya, dalam satu transaksi.
func (c *ContactUseCase) Delete(ctx context.Context, req *model.DeleteContactRequest) error {
	if err := c.validate.Struct(req); err != nil {
		return err
	}
	return c.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		contact, err := c.contacts.FindByIDAndUserID(ctx, req.ID, req.UserID)
		if err != nil {
			return err
		}
		if err := c.addresses.DeleteAllByContactID(ctx, contact.ID); err != nil {
			return err
		}
		return c.contacts.Delete(ctx, contact)
	})
}

// Search mencari kontak milik user dengan filter dan paging.
func (c *ContactUseCase) Search(ctx context.Context, req *model.SearchContactRequest) ([]model.ContactResponse, *model.PageMetadata, error) {
	if err := c.validate.Struct(req); err != nil {
		return nil, nil, err
	}
	contacts, total, err := c.contacts.Search(ctx, req)
	if err != nil {
		return nil, nil, err
	}

	responses := make([]model.ContactResponse, len(contacts))
	for i := range contacts {
		responses[i] = *converter.ContactToResponse(&contacts[i])
	}
	return responses, model.NewPageMetadata(req.Page, req.Size, total), nil
}
