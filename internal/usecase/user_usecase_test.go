package usecase_test

import (
	"context"
	"testing"

	"github.com/primestrider/golang-clean-architecture/internal/apperror"
	"github.com/primestrider/golang-clean-architecture/internal/entity"
	"github.com/primestrider/golang-clean-architecture/internal/model"
	"github.com/primestrider/golang-clean-architecture/internal/security"
	"github.com/primestrider/golang-clean-architecture/internal/usecase"
	"github.com/primestrider/golang-clean-architecture/internal/usecase/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type userFixture struct {
	users  *mocks.UserRepository
	tokens *mocks.RefreshTokenRepository
	events *mocks.EventPublisher
	uc     *usecase.UserUseCase
}

func newUserFixture(t *testing.T) *userFixture {
	f := &userFixture{
		users:  mocks.NewUserRepository(t),
		tokens: mocks.NewRefreshTokenRepository(t),
		events: mocks.NewEventPublisher(t),
	}
	f.uc = usecase.NewUserUseCase(discardLogger(), newValidator(), &fakeTransactor{}, f.users, f.tokens, f.events)
	return f
}

func fieldsOf(err error) []string {
	appErr, ok := apperror.As(err)
	if !ok {
		return nil
	}
	var names []string
	for _, f := range appErr.Fields {
		names = append(names, f.Field)
	}
	return names
}

func TestRegister_NormalizesEmail(t *testing.T) {
	f := newUserFixture(t)
	f.users.EXPECT().Create(mock.Anything, mock.MatchedBy(func(u *entity.User) bool {
		return u.Email == "eko@example.com" && u.Role == "user" && u.ID != "" && u.Password != "rahasia123"
	})).Return(nil)
	f.events.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil)

	res, err := f.uc.Create(context.Background(), &model.RegisterUserRequest{Email: "  Eko@Example.COM ", Password: "rahasia123", Name: "Eko"})

	require.NoError(t, err)
	assert.Equal(t, "eko@example.com", res.Email)
	assert.Equal(t, "user", res.Role)
}

func TestRegister_PasswordTooShort(t *testing.T) {
	f := newUserFixture(t)

	_, err := f.uc.Create(context.Background(), &model.RegisterUserRequest{Email: "eko@example.com", Password: "short", Name: "Eko"})

	assert.Equal(t, []string{"password"}, fieldsOf(err))
}

func TestRegister_PasswordLongerThan72BytesRejected(t *testing.T) {
	f := newUserFixture(t)
	long := make([]byte, 73)
	for i := range long {
		long[i] = 'a'
	}

	_, err := f.uc.Create(context.Background(), &model.RegisterUserRequest{Email: "eko@example.com", Password: string(long), Name: "Eko"})

	assert.Equal(t, []string{"password"}, fieldsOf(err))
}

func TestRegister_DuplicateEmailIsConflict(t *testing.T) {
	f := newUserFixture(t)
	f.users.EXPECT().Create(mock.Anything, mock.Anything).Return(apperror.Conflict("email already registered"))

	_, err := f.uc.Create(context.Background(), &model.RegisterUserRequest{Email: "eko@example.com", Password: "rahasia123", Name: "Eko"})

	assert.Equal(t, apperror.CodeConflict, codeOf(err))
}

func existingUser(t *testing.T, password string) *entity.User {
	hash, err := security.HashPassword(password)
	require.NoError(t, err)
	return &entity.User{ID: "u1", Email: "eko@example.com", Name: "Eko", Password: hash, Role: "user", Version: 1}
}

func TestUpdate_NewPasswordRequiresCurrent(t *testing.T) {
	f := newUserFixture(t)
	f.users.EXPECT().FindByID(mock.Anything, "u1").Return(existingUser(t, "rahasia123"), nil)

	_, err := f.uc.Update(context.Background(), &model.UpdateUserRequest{ID: "u1", NewPassword: "passwordbaru"})

	assert.Equal(t, apperror.CodeValidation, codeOf(err))
	assert.Equal(t, []string{"current_password"}, fieldsOf(err))
}

func TestUpdate_WrongCurrentPasswordRejected(t *testing.T) {
	f := newUserFixture(t)
	f.users.EXPECT().FindByID(mock.Anything, "u1").Return(existingUser(t, "rahasia123"), nil)

	_, err := f.uc.Update(context.Background(), &model.UpdateUserRequest{ID: "u1", CurrentPassword: "salah", NewPassword: "passwordbaru"})

	assert.Equal(t, []string{"current_password"}, fieldsOf(err))
}

func TestUpdate_PasswordChangeRevokesAllRefreshTokens(t *testing.T) {
	f := newUserFixture(t)
	f.users.EXPECT().FindByID(mock.Anything, "u1").Return(existingUser(t, "rahasia123"), nil)
	f.users.EXPECT().Update(mock.Anything, mock.MatchedBy(func(u *entity.User) bool {
		return security.ComparePassword(u.Password, "passwordbaru")
	})).Return(nil)
	f.tokens.EXPECT().RevokeAllForUser(mock.Anything, "u1").Return(nil).Once()
	f.events.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil)

	_, err := f.uc.Update(context.Background(), &model.UpdateUserRequest{ID: "u1", CurrentPassword: "rahasia123", NewPassword: "passwordbaru"})

	require.NoError(t, err)
}

func TestUpdate_NameOnlyDoesNotRevokeTokens(t *testing.T) {
	f := newUserFixture(t)
	f.users.EXPECT().FindByID(mock.Anything, "u1").Return(existingUser(t, "rahasia123"), nil)
	f.users.EXPECT().Update(mock.Anything, mock.Anything).Return(nil)
	f.events.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil)

	res, err := f.uc.Update(context.Background(), &model.UpdateUserRequest{ID: "u1", Name: "Eko Baru"})

	require.NoError(t, err)
	assert.Equal(t, "Eko Baru", res.Name)
	// tokens.RevokeAllForUser tidak di-EXPECT, jadi pemanggilan akan
	// menggagalkan test.
}
