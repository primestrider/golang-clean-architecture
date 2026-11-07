package grpc

import (
	"context"

	contactv1 "github.com/primestrider/golang-clean-architecture/internal/delivery/grpc/pb/contact/v1"
	"github.com/primestrider/golang-clean-architecture/internal/model"
	"github.com/primestrider/golang-clean-architecture/internal/usecase"
)

// ContactService mengimplementasikan contactv1.ContactServiceServer di atas
// ContactUseCase.
type ContactService struct {
	contactv1.UnimplementedContactServiceServer
	contacts *usecase.ContactUseCase
}

// CreateContact membuat kontak untuk user yang login.
func (s *ContactService) CreateContact(ctx context.Context, req *contactv1.CreateContactRequest) (*contactv1.CreateContactResponse, error) {
	res, err := s.contacts.Create(ctx, &model.CreateContactRequest{
		UserID:    currentAuth(ctx).UserID,
		FirstName: req.GetFirstName(),
		LastName:  req.GetLastName(),
		Email:     req.GetEmail(),
		Phone:     req.GetPhone(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &contactv1.CreateContactResponse{Contact: toProto(res)}, nil
}

// GetContact mengambil satu kontak milik user yang login.
func (s *ContactService) GetContact(ctx context.Context, req *contactv1.GetContactRequest) (*contactv1.GetContactResponse, error) {
	res, err := s.contacts.Get(ctx, &model.GetContactRequest{UserID: currentAuth(ctx).UserID, ID: req.GetId()})
	if err != nil {
		return nil, toStatus(err)
	}
	return &contactv1.GetContactResponse{Contact: toProto(res)}, nil
}

// UpdateContact mengganti isi kontak dengan optimistic locking.
func (s *ContactService) UpdateContact(ctx context.Context, req *contactv1.UpdateContactRequest) (*contactv1.UpdateContactResponse, error) {
	res, err := s.contacts.Update(ctx, &model.UpdateContactRequest{
		UserID:    currentAuth(ctx).UserID,
		ID:        req.GetId(),
		FirstName: req.GetFirstName(),
		LastName:  req.GetLastName(),
		Email:     req.GetEmail(),
		Phone:     req.GetPhone(),
		Version:   int(req.GetVersion()),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &contactv1.UpdateContactResponse{Contact: toProto(res)}, nil
}

// DeleteContact menghapus kontak (soft delete).
func (s *ContactService) DeleteContact(ctx context.Context, req *contactv1.DeleteContactRequest) (*contactv1.DeleteContactResponse, error) {
	if err := s.contacts.Delete(ctx, &model.DeleteContactRequest{UserID: currentAuth(ctx).UserID, ID: req.GetId()}); err != nil {
		return nil, toStatus(err)
	}
	return &contactv1.DeleteContactResponse{}, nil
}

// ListContacts mencari kontak dengan filter dan paging. page dan size
// bernilai 0 diganti default (1 dan 10), sama seperti REST API.
func (s *ContactService) ListContacts(ctx context.Context, req *contactv1.ListContactsRequest) (*contactv1.ListContactsResponse, error) {
	page, size := int(req.GetPage()), int(req.GetSize())
	if page == 0 {
		page = 1
	}
	if size == 0 {
		size = 10
	}
	res, paging, err := s.contacts.Search(ctx, &model.SearchContactRequest{
		UserID: currentAuth(ctx).UserID,
		Name:   req.GetName(),
		Email:  req.GetEmail(),
		Phone:  req.GetPhone(),
		Page:   page,
		Size:   size,
	})
	if err != nil {
		return nil, toStatus(err)
	}
	contacts := make([]*contactv1.Contact, len(res))
	for i := range res {
		contacts[i] = toProto(&res[i])
	}
	return &contactv1.ListContactsResponse{
		Contacts:  contacts,
		Page:      int32(paging.Page), //nolint:gosec // dibatasi validasi (maks 100 per halaman)
		Size:      int32(paging.Size), //nolint:gosec // dibatasi validasi
		TotalItem: paging.TotalItem,
		TotalPage: paging.TotalPage,
	}, nil
}

// toProto mengubah model respons menjadi pesan proto.
func toProto(c *model.ContactResponse) *contactv1.Contact {
	return &contactv1.Contact{
		Id:        c.ID,
		FirstName: c.FirstName,
		LastName:  c.LastName,
		Email:     c.Email,
		Phone:     c.Phone,
		Version:   int32(c.Version), //nolint:gosec // versi bertambah satu per update, jauh di bawah batas int32
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}
