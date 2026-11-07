package usecase

import (
	"context"
	"log/slog"
	"time"

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
	audit     *AuditRecorder
	cache     Cache
}

// NewContactUseCase membuat ContactUseCase dengan dependency-nya.
func NewContactUseCase(log *slog.Logger, validate *validation.Validator, tx Transactor,
	contacts ContactRepository, addresses AddressRepository, events EventPublisher, audit *AuditRecorder, cache Cache) *ContactUseCase {
	return &ContactUseCase{log: log, validate: validate, tx: tx, contacts: contacts, addresses: addresses, events: events, audit: audit, cache: cache}
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
		if err := c.audit.Record(ctx, "contact.created", "contact", contact.ID, nil, converter.ContactToResponse(contact)); err != nil {
			return err
		}
		return publish(ctx, c.events, "contact.created", contact.ID, converter.ContactToResponse(contact))
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
		before := converter.ContactToResponse(contact)
		contact.FirstName = req.FirstName
		contact.LastName = req.LastName
		contact.Email = req.Email
		contact.Phone = req.Phone
		if err := c.contacts.UpdateWithVersion(ctx, contact, req.Version); err != nil {
			return err
		}
		if err := c.audit.Record(ctx, "contact.updated", "contact", contact.ID, before, converter.ContactToResponse(contact)); err != nil {
			return err
		}
		return publish(ctx, c.events, "contact.updated", contact.ID, converter.ContactToResponse(contact))
	})
	if err != nil {
		return nil, err
	}
	c.invalidate(ctx, req.UserID, req.ID)
	return converter.ContactToResponse(contact), nil
}

// Get mengambil satu kontak milik user yang login. Operasi baca tidak
// membuka transaksi.
func (c *ContactUseCase) Get(ctx context.Context, req *model.GetContactRequest) (*model.ContactResponse, error) {
	if err := c.validate.Struct(req); err != nil {
		return nil, err
	}
	key := contactCacheKey(req.UserID, req.ID)
	cached := new(model.ContactResponse)
	if hit, err := c.cache.Get(ctx, key, cached); err != nil {
		c.log.WarnContext(ctx, "cache get failed, falling back to database", "key", key, "error", err)
	} else if hit {
		return cached, nil
	}

	contact, err := c.contacts.FindByIDAndUserID(ctx, req.ID, req.UserID)
	if err != nil {
		return nil, err
	}
	res := converter.ContactToResponse(contact)
	if err := c.cache.Set(ctx, key, res, contactCacheTTL); err != nil {
		c.log.WarnContext(ctx, "cache set failed", "key", key, "error", err)
	}
	return res, nil
}

// Delete men-soft-delete kontak milik user yang login beserta semua
// alamatnya, dalam satu transaksi.
func (c *ContactUseCase) Delete(ctx context.Context, req *model.DeleteContactRequest) error {
	if err := c.validate.Struct(req); err != nil {
		return err
	}
	err := c.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		contact, err := c.contacts.FindByIDAndUserID(ctx, req.ID, req.UserID)
		if err != nil {
			return err
		}
		if err := c.addresses.DeleteAllByContactID(ctx, contact.ID); err != nil {
			return err
		}
		if err := c.contacts.Delete(ctx, contact); err != nil {
			return err
		}
		if err := c.audit.Record(ctx, "contact.deleted", "contact", contact.ID, converter.ContactToResponse(contact), nil); err != nil {
			return err
		}
		return publish(ctx, c.events, "contact.deleted", contact.ID, converter.ContactToResponse(contact))
	})
	if err != nil {
		return err
	}
	c.invalidate(ctx, req.UserID, req.ID)
	return nil
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

// contactCacheTTL adalah umur cache satu kontak. Cache tetap dihapus setiap
// kali kontak berubah; TTL hanya jaring pengaman bila penghapusan gagal.
const contactCacheTTL = 5 * time.Minute

// contactCacheKey membentuk key cache kontak. User id ikut di key supaya
// cache satu user tidak pernah terbaca oleh user lain.
func contactCacheKey(userID, contactID string) string {
	return "contact:" + userID + ":" + contactID
}

// invalidate menghapus cache kontak SETELAH transaksi commit. Bila dihapus
// sebelum commit, request lain bisa mengisi cache dengan data lama sebelum
// data baru tersimpan. Kegagalan hanya dicatat, tidak menggagalkan request.
func (c *ContactUseCase) invalidate(ctx context.Context, userID, contactID string) {
	if err := c.cache.Delete(ctx, contactCacheKey(userID, contactID)); err != nil {
		c.log.WarnContext(ctx, "cache invalidation failed", "contact_id", contactID, "error", err)
	}
}
