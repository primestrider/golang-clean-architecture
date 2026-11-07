package converter

import (
	"github.com/primestrider/golang-clean-architecture/internal/entity"
	"github.com/primestrider/golang-clean-architecture/internal/model"
)

func ContactToResponse(contact *entity.Contact) *model.ContactResponse {
	return &model.ContactResponse{
		ID:        contact.ID,
		FirstName: contact.FirstName,
		LastName:  contact.LastName,
		Email:     contact.Email,
		Phone:     contact.Phone,
		CreatedAt: contact.CreatedAt,
		UpdatedAt: contact.UpdatedAt,
	}
}

func ContactToEvent(contact *entity.Contact) *model.ContactEvent {
	return &model.ContactEvent{
		ID:        contact.ID,
		UserID:    contact.UserId,
		FirstName: contact.FirstName,
		LastName:  contact.LastName,
		Email:     contact.Email,
		Phone:     contact.Phone,
		CreatedAt: contact.CreatedAt,
		UpdatedAt: contact.UpdatedAt,
	}
}
