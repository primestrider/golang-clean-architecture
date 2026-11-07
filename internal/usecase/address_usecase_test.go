package usecase_test

import (
	"context"
	"testing"

	"github.com/primestrider/golang-clean-architecture/internal/apperror"
	"github.com/primestrider/golang-clean-architecture/internal/model"
	"github.com/primestrider/golang-clean-architecture/internal/usecase"
	"github.com/primestrider/golang-clean-architecture/internal/usecase/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const contactID = "6f1c1f0e-4b7a-4c64-9d7e-6a0b7a1d2c3e"

func TestAddressCreate_ContactNotOwned(t *testing.T) {
	contacts := mocks.NewContactRepository(t)
	addresses := mocks.NewAddressRepository(t)
	events := mocks.NewEventPublisher(t)
	uc := usecase.NewAddressUseCase(discardLogger(), newValidator(), &fakeTransactor{}, contacts, addresses, events, newAudit())
	contacts.EXPECT().FindByIDAndUserID(mock.Anything, contactID, "intruder").Return(nil, apperror.NotFound("contact"))

	_, err := uc.Create(context.Background(), &model.CreateAddressRequest{UserID: "intruder", ContactID: contactID, City: "Jakarta"})

	assert.Equal(t, apperror.CodeNotFound, codeOf(err))
	// addresses dan events tidak boleh dipanggil sama sekali.
}
