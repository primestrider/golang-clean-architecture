package usecase_test

import (
	"context"
	"testing"

	"github.com/primestrider/golang-clean-architecture/internal/apperror"
	"github.com/primestrider/golang-clean-architecture/internal/entity"
	"github.com/primestrider/golang-clean-architecture/internal/model"
	"github.com/primestrider/golang-clean-architecture/internal/usecase"
	"github.com/primestrider/golang-clean-architecture/internal/usecase/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type contactFixture struct {
	tx        *fakeTransactor
	contacts  *mocks.ContactRepository
	addresses *mocks.AddressRepository
	events    *mocks.EventPublisher
	uc        *usecase.ContactUseCase
}

func newContactFixture(t *testing.T) *contactFixture {
	f := &contactFixture{
		tx:        &fakeTransactor{},
		contacts:  mocks.NewContactRepository(t),
		addresses: mocks.NewAddressRepository(t),
		events:    mocks.NewEventPublisher(t),
	}
	f.uc = usecase.NewContactUseCase(discardLogger(), newValidator(), f.tx, f.contacts, f.addresses, f.events)
	return f
}

func TestContactCreate_ValidationError(t *testing.T) {
	f := newContactFixture(t)

	_, err := f.uc.Create(context.Background(), &model.CreateContactRequest{UserID: "u1", Email: "not-an-email"})

	assert.Equal(t, apperror.CodeValidation, codeOf(err))
	// Mock dibuat dengan NewContactRepository(t): pemanggilan yang tidak
	// diharapkan otomatis menggagalkan test, jadi repository terbukti tidak
	// disentuh.
}

func TestContactGet_InvalidIDIsValidationError(t *testing.T) {
	f := newContactFixture(t)

	_, err := f.uc.Get(context.Background(), &model.GetContactRequest{UserID: "u1", ID: "c1"})

	assert.Equal(t, apperror.CodeValidation, codeOf(err), "id must be a UUID")
}

func TestContactGet_NotOwnedIsNotFound(t *testing.T) {
	f := newContactFixture(t)
	id := "6f1c1f0e-4b7a-4c64-9d7e-6a0b7a1d2c3e"
	f.contacts.EXPECT().FindByIDAndUserID(mock.Anything, id, "u1").Return(nil, apperror.NotFound("contact"))

	_, err := f.uc.Get(context.Background(), &model.GetContactRequest{UserID: "u1", ID: id})

	assert.Equal(t, apperror.CodeNotFound, codeOf(err))
}

func TestContactCreate_Success_PublishesEventInTransaction(t *testing.T) {
	f := newContactFixture(t)
	f.contacts.EXPECT().Create(mock.Anything, mock.AnythingOfType("*entity.Contact")).Return(nil)
	f.events.EXPECT().Publish(mock.Anything, mock.Anything).RunAndReturn(func(context.Context, ...model.Event) error {
		assert.True(t, f.tx.inTx, "event must be published inside the transaction")
		return nil
	}).Once()

	res, err := f.uc.Create(context.Background(), &model.CreateContactRequest{UserID: "u1", FirstName: "Eko", Email: "eko@example.com"})

	require.NoError(t, err)
	assert.Equal(t, "Eko", res.FirstName)
	assert.NotEmpty(t, res.ID)
}

func TestContactCreate_PublishFailureReturnsError(t *testing.T) {
	f := newContactFixture(t)
	f.contacts.EXPECT().Create(mock.Anything, mock.Anything).Return(nil)
	f.events.EXPECT().Publish(mock.Anything, mock.Anything).Return(assert.AnError)

	_, err := f.uc.Create(context.Background(), &model.CreateContactRequest{UserID: "u1", FirstName: "Eko"})

	assert.ErrorIs(t, err, assert.AnError)
}

func TestContactSearch_BuildsPaging(t *testing.T) {
	f := newContactFixture(t)
	f.contacts.EXPECT().Search(mock.Anything, mock.Anything).Return([]entity.Contact{{ID: "a"}, {ID: "b"}}, int64(21), nil)

	res, paging, err := f.uc.Search(context.Background(), &model.SearchContactRequest{UserID: "u1", Page: 2, Size: 10})

	require.NoError(t, err)
	assert.Len(t, res, 2)
	assert.Equal(t, &model.PageMetadata{Page: 2, Size: 10, TotalItem: 21, TotalPage: 3}, paging)
}
