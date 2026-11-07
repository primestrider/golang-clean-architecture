package usecase_test

import (
	"context"
	"testing"
	"time"

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

var authNow = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

type authFixture struct {
	users      *mocks.UserRepository
	tokens     *mocks.RefreshTokenRepository
	issuer     *mocks.TokenIssuer
	dummyCalls int
	uc         *usecase.AuthUseCase
}

func newAuthFixture(t *testing.T) *authFixture {
	f := &authFixture{
		users:  mocks.NewUserRepository(t),
		tokens: mocks.NewRefreshTokenRepository(t),
		issuer: mocks.NewTokenIssuer(t),
	}
	f.uc = usecase.NewAuthUseCase(usecase.AuthDeps{
		Log:          discardLogger(),
		Validate:     newValidator(),
		Tx:           &fakeTransactor{},
		Users:        f.users,
		Tokens:       f.tokens,
		Issuer:       f.issuer,
		RefreshTTL:   7 * 24 * time.Hour,
		Now:          func() time.Time { return authNow },
		CompareDummy: func(string) { f.dummyCalls++ },
	})
	return f
}

func (f *authFixture) expectIssue() {
	f.issuer.EXPECT().Issue(mock.Anything, mock.Anything).Return("access-jwt", authNow.Add(15*time.Minute), nil)
}

func TestLogin_Success(t *testing.T) {
	f := newAuthFixture(t)
	f.users.EXPECT().FindByEmail(mock.Anything, "eko@example.com").Return(existingUser(t, "rahasia123"), nil)
	f.expectIssue()
	f.tokens.EXPECT().Create(mock.Anything, mock.MatchedBy(func(rt *entity.RefreshToken) bool {
		return rt.UserID == "u1" && rt.FamilyID != "" && len(rt.TokenHash) == 64 &&
			rt.ExpiresAt == authNow.Add(7*24*time.Hour).UnixMilli()
	})).Return(nil)

	res, err := f.uc.Login(context.Background(), &model.LoginRequest{Email: "Eko@Example.com", Password: "rahasia123"})

	require.NoError(t, err)
	assert.Equal(t, "access-jwt", res.AccessToken)
	assert.Equal(t, "Bearer", res.TokenType)
	assert.Equal(t, 900, res.ExpiresIn)
	assert.Len(t, res.RefreshToken, 43)
}

func TestLogin_UnknownEmail_SameErrorAsWrongPassword(t *testing.T) {
	f := newAuthFixture(t)
	f.users.EXPECT().FindByEmail(mock.Anything, "ghost@example.com").Return(nil, apperror.NotFound("user"))
	f.users.EXPECT().FindByEmail(mock.Anything, "eko@example.com").Return(existingUser(t, "rahasia123"), nil)

	_, errUnknown := f.uc.Login(context.Background(), &model.LoginRequest{Email: "ghost@example.com", Password: "whatever1"})
	_, errWrong := f.uc.Login(context.Background(), &model.LoginRequest{Email: "eko@example.com", Password: "wrongpass"})

	unknown, _ := apperror.As(errUnknown)
	wrong, _ := apperror.As(errWrong)
	require.NotNil(t, unknown)
	require.NotNil(t, wrong)
	assert.Equal(t, apperror.CodeUnauthorized, unknown.Code)
	assert.Equal(t, "invalid email or password", unknown.Message)
	assert.Equal(t, wrong.Message, unknown.Message)
	assert.Equal(t, 1, f.dummyCalls, "unknown email must still spend a bcrypt comparison")
}

func storedToken(plain string, revoked bool, expiresAt time.Time) *entity.RefreshToken {
	rt := &entity.RefreshToken{
		ID: "rt1", UserID: "u1", FamilyID: "fam1",
		TokenHash: security.HashRefreshToken(plain), ExpiresAt: expiresAt.UnixMilli(),
	}
	if revoked {
		at := authNow.Add(-time.Minute).UnixMilli()
		rt.RevokedAt = &at
	}
	return rt
}

func TestRefresh_RotatesToken(t *testing.T) {
	f := newAuthFixture(t)
	plain := "old-refresh-token"
	f.tokens.EXPECT().FindByHashForUpdate(mock.Anything, security.HashRefreshToken(plain)).
		Return(storedToken(plain, false, authNow.Add(time.Hour)), nil)
	f.users.EXPECT().FindByID(mock.Anything, "u1").Return(existingUser(t, "x"), nil)
	var newID string
	f.tokens.EXPECT().Create(mock.Anything, mock.MatchedBy(func(rt *entity.RefreshToken) bool {
		newID = rt.ID
		return rt.FamilyID == "fam1" && rt.UserID == "u1"
	})).Return(nil)
	f.tokens.EXPECT().Revoke(mock.Anything, "rt1", mock.MatchedBy(func(id *string) bool {
		return id != nil && *id == newID
	})).Return(nil)
	f.expectIssue()

	res, err := f.uc.Refresh(context.Background(), &model.RefreshRequest{RefreshToken: plain})

	require.NoError(t, err)
	assert.NotEqual(t, plain, res.RefreshToken)
}

func TestRefresh_ReusedTokenRevokesFamily(t *testing.T) {
	f := newAuthFixture(t)
	plain := "stolen-refresh-token"
	f.tokens.EXPECT().FindByHashForUpdate(mock.Anything, mock.Anything).
		Return(storedToken(plain, true, authNow.Add(time.Hour)), nil)
	f.tokens.EXPECT().RevokeFamily(mock.Anything, "fam1").Return(nil).Once()

	_, err := f.uc.Refresh(context.Background(), &model.RefreshRequest{RefreshToken: plain})

	assert.Equal(t, apperror.CodeUnauthorized, codeOf(err))
}

func TestRefresh_Expired(t *testing.T) {
	f := newAuthFixture(t)
	f.tokens.EXPECT().FindByHashForUpdate(mock.Anything, mock.Anything).
		Return(storedToken("t", false, authNow.Add(-time.Second)), nil)

	_, err := f.uc.Refresh(context.Background(), &model.RefreshRequest{RefreshToken: "t"})

	assert.Equal(t, apperror.CodeUnauthorized, codeOf(err))
}

func TestRefresh_UnknownToken(t *testing.T) {
	f := newAuthFixture(t)
	f.tokens.EXPECT().FindByHashForUpdate(mock.Anything, mock.Anything).Return(nil, apperror.NotFound("refresh token"))

	_, err := f.uc.Refresh(context.Background(), &model.RefreshRequest{RefreshToken: "nope"})

	assert.Equal(t, apperror.CodeUnauthorized, codeOf(err))
}

func TestLogout_RevokesFamily(t *testing.T) {
	f := newAuthFixture(t)
	f.tokens.EXPECT().FindByHashForUpdate(mock.Anything, mock.Anything).
		Return(storedToken("t", false, authNow.Add(time.Hour)), nil)
	f.tokens.EXPECT().RevokeFamily(mock.Anything, "fam1").Return(nil).Once()

	assert.NoError(t, f.uc.Logout(context.Background(), &model.LogoutRequest{RefreshToken: "t"}))
}

func TestLogout_UnknownTokenIsNoop(t *testing.T) {
	f := newAuthFixture(t)
	f.tokens.EXPECT().FindByHashForUpdate(mock.Anything, mock.Anything).Return(nil, apperror.NotFound("refresh token"))

	assert.NoError(t, f.uc.Logout(context.Background(), &model.LogoutRequest{RefreshToken: "t"}))
}
