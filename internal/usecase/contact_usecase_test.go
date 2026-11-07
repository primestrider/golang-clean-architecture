package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

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
	cache     *mocks.Cache
	uc        *usecase.ContactUseCase
}

func newContactFixture(t *testing.T) *contactFixture {
	f := &contactFixture{
		tx:        &fakeTransactor{},
		contacts:  mocks.NewContactRepository(t),
		addresses: mocks.NewAddressRepository(t),
		events:    mocks.NewEventPublisher(t),
		cache:     mocks.NewCache(t),
	}
	f.uc = usecase.NewContactUseCase(discardLogger(), newValidator(), f.tx, f.contacts, f.addresses, f.events, newAudit(), f.cache)
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
	f.cache.EXPECT().Get(mock.Anything, mock.Anything, mock.Anything).Return(false, nil)
	f.contacts.EXPECT().FindByIDAndUserID(mock.Anything, id, "u1").Return(nil, apperror.NotFound("contact"))

	_, err := f.uc.Get(context.Background(), &model.GetContactRequest{UserID: "u1", ID: id})

	assert.Equal(t, apperror.CodeNotFound, codeOf(err))
}

func TestContactCreate_Success_PublishesEventInTransaction(t *testing.T) {
	f := newContactFixture(t)
	f.contacts.EXPECT().Create(mock.Anything, mock.AnythingOfType("*entity.Contact")).Return(nil)
	f.events.EXPECT().Publish(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, events ...model.Event) error {
		assert.True(t, f.tx.inTx, "event must be published inside the transaction")
		require.Len(t, events, 1)
		assert.Equal(t, "contact.created", events[0].EventType)
		assert.NotContains(t, string(events[0].Data), "user_id")
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

const cachedContactID = "6f1c1f0e-4b7a-4c64-9d7e-6a0b7a1d2c3e"

func TestContactGet_CacheHitSkipsRepo(t *testing.T) {
	f := newContactFixture(t)
	f.cache.EXPECT().Get(mock.Anything, "contact:u1:"+cachedContactID, mock.Anything).
		RunAndReturn(func(_ context.Context, _ string, dst any) (bool, error) {
			*dst.(*model.ContactResponse) = model.ContactResponse{ID: cachedContactID, FirstName: "Cached"}
			return true, nil
		})

	res, err := f.uc.Get(context.Background(), &model.GetContactRequest{UserID: "u1", ID: cachedContactID})

	require.NoError(t, err)
	assert.Equal(t, "Cached", res.FirstName)
	// contacts.FindByIDAndUserID tidak di-EXPECT: repo tidak boleh dipanggil.
}

func TestContactGet_CacheMissLoadsAndStores(t *testing.T) {
	f := newContactFixture(t)
	f.cache.EXPECT().Get(mock.Anything, mock.Anything, mock.Anything).Return(false, nil)
	f.contacts.EXPECT().FindByIDAndUserID(mock.Anything, cachedContactID, "u1").Return(&entity.Contact{ID: cachedContactID, FirstName: "Budi"}, nil)
	f.cache.EXPECT().Set(mock.Anything, "contact:u1:"+cachedContactID, mock.Anything, 5*time.Minute).Return(nil).Once()

	res, err := f.uc.Get(context.Background(), &model.GetContactRequest{UserID: "u1", ID: cachedContactID})

	require.NoError(t, err)
	assert.Equal(t, "Budi", res.FirstName)
}

func TestContactGet_CacheErrorFallsBackToRepo(t *testing.T) {
	f := newContactFixture(t)
	f.cache.EXPECT().Get(mock.Anything, mock.Anything, mock.Anything).Return(false, errors.New("redis down"))
	f.contacts.EXPECT().FindByIDAndUserID(mock.Anything, cachedContactID, "u1").Return(&entity.Contact{ID: cachedContactID}, nil)
	f.cache.EXPECT().Set(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(errors.New("redis down"))

	_, err := f.uc.Get(context.Background(), &model.GetContactRequest{UserID: "u1", ID: cachedContactID})

	assert.NoError(t, err, "cache failures must never fail the request")
}

func TestContactUpdate_InvalidatesCacheAfterCommit(t *testing.T) {
	f := newContactFixture(t)
	f.contacts.EXPECT().FindByIDAndUserID(mock.Anything, cachedContactID, "u1").Return(&entity.Contact{ID: cachedContactID, Version: 1}, nil)
	f.contacts.EXPECT().UpdateWithVersion(mock.Anything, mock.Anything, 1).Return(nil)
	f.events.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil)
	f.cache.EXPECT().Delete(mock.Anything, []string{"contact:u1:" + cachedContactID}).RunAndReturn(func(context.Context, ...string) error {
		assert.False(t, f.tx.inTx, "cache must be invalidated after the transaction commits")
		return nil
	}).Once()

	_, err := f.uc.Update(context.Background(), &model.UpdateContactRequest{UserID: "u1", ID: cachedContactID, FirstName: "Baru", Version: 1})

	require.NoError(t, err)
}
