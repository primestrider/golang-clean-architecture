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
)

func TestUserRegister_Duplicate(t *testing.T) {
	users := mocks.NewUserRepository(t)
	uc := usecase.NewUserUseCase(discardLogger(), newValidator(), &fakeTransactor{}, users, mocks.NewEventPublisher(t))
	users.EXPECT().FindByID(mock.Anything, "eko").Return(&entity.User{ID: "eko"}, nil)

	_, err := uc.Create(context.Background(), &model.RegisterUserRequest{ID: "eko", Password: "rahasia123", Name: "Eko"})

	assert.Equal(t, apperror.CodeConflict, codeOf(err))
}

func TestUserLogin_UnknownUserIsUnauthorized(t *testing.T) {
	users := mocks.NewUserRepository(t)
	uc := usecase.NewUserUseCase(discardLogger(), newValidator(), &fakeTransactor{}, users, mocks.NewEventPublisher(t))
	users.EXPECT().FindByID(mock.Anything, "ghost").Return(nil, apperror.NotFound("user"))

	_, err := uc.Login(context.Background(), &model.LoginUserRequest{ID: "ghost", Password: "whatever"})

	assert.Equal(t, apperror.CodeUnauthorized, codeOf(err))
}
