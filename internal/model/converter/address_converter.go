package converter

import (
	"github.com/primestrider/golang-clean-architecture/internal/entity"
	"github.com/primestrider/golang-clean-architecture/internal/model"
)

func AddressToResponse(address *entity.Address) *model.AddressResponse {
	return &model.AddressResponse{
		ID:         address.ID,
		Street:     address.Street,
		City:       address.City,
		Province:   address.Province,
		PostalCode: address.PostalCode,
		Country:    address.Country,
		CreatedAt:  address.CreatedAt,
		UpdatedAt:  address.UpdatedAt,
	}
}

func AddressToEvent(address *entity.Address) *model.AddressEvent {
	return &model.AddressEvent{
		ID:         address.ID,
		ContactId:  address.ContactId,
		Street:     address.Street,
		City:       address.City,
		Province:   address.Province,
		PostalCode: address.PostalCode,
		Country:    address.Country,
		CreatedAt:  address.CreatedAt,
		UpdatedAt:  address.UpdatedAt,
	}
}
