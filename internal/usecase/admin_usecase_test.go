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

func newAdminUseCase(t *testing.T) (*usecase.AdminUseCase, *mocks.UserRepository, *mocks.EventPublisher) {
	users := mocks.NewUserRepository(t)
	events := mocks.NewEventPublisher(t)
	return usecase.NewAdminUseCase(discardLogger(), newValidator(), &fakeTransactor{}, users, events), users, events
}

func TestChangeRole_CannotDemoteSelf(t *testing.T) {
	uc, _, _ := newAdminUseCase(t)

	_, err := uc.ChangeRole(context.Background(), &model.ChangeRoleRequest{ActorID: "admin-1", UserID: "admin-1", Role: "user"})

	assert.Equal(t, apperror.CodeConflict, codeOf(err))
}

func TestChangeRole_InvalidRole(t *testing.T) {
	uc, _, _ := newAdminUseCase(t)

	_, err := uc.ChangeRole(context.Background(), &model.ChangeRoleRequest{ActorID: "admin-1", UserID: "u2", Role: "superuser"})

	assert.Equal(t, []string{"role"}, fieldsOf(err))
}

func TestChangeRole_PromotesUser(t *testing.T) {
	uc, users, events := newAdminUseCase(t)
	users.EXPECT().FindByID(mock.Anything, "u2").Return(&entity.User{ID: "u2", Role: entity.RoleUser}, nil)
	users.EXPECT().Update(mock.Anything, mock.MatchedBy(func(u *entity.User) bool { return u.Role == entity.RoleAdmin })).Return(nil)
	events.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil)

	res, err := uc.ChangeRole(context.Background(), &model.ChangeRoleRequest{ActorID: "admin-1", UserID: "u2", Role: "admin"})

	require.NoError(t, err)
	assert.Equal(t, "admin", res.Role)
}

func TestListUsers_Paging(t *testing.T) {
	uc, users, _ := newAdminUseCase(t)
	users.EXPECT().List(mock.Anything, 1, 2).Return([]entity.User{{ID: "a"}, {ID: "b"}}, int64(5), nil)

	res, paging, err := uc.ListUsers(context.Background(), &model.ListUsersRequest{Page: 1, Size: 2})

	require.NoError(t, err)
	assert.Len(t, res, 2)
	assert.Equal(t, int64(3), paging.TotalPage)
}

func TestPromoteByEmail_NotFound(t *testing.T) {
	uc, users, _ := newAdminUseCase(t)
	users.EXPECT().FindByEmail(mock.Anything, "ghost@example.com").Return(nil, apperror.NotFound("user"))

	_, err := uc.PromoteByEmail(context.Background(), " Ghost@Example.com ")

	assert.Equal(t, apperror.CodeNotFound, codeOf(err))
}

func TestPromoteByEmail_SetsAdminRole(t *testing.T) {
	uc, users, events := newAdminUseCase(t)
	users.EXPECT().FindByEmail(mock.Anything, "eko@example.com").Return(&entity.User{ID: "u1", Role: entity.RoleUser}, nil)
	users.EXPECT().Update(mock.Anything, mock.MatchedBy(func(u *entity.User) bool { return u.Role == entity.RoleAdmin })).Return(nil)
	events.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil)

	res, err := uc.PromoteByEmail(context.Background(), "eko@example.com")

	require.NoError(t, err)
	assert.Equal(t, "admin", res.Role)
}
