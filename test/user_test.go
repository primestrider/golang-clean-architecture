//go:build integration

package test

import (
	"net/http"
	"testing"

	"github.com/primestrider/golang-clean-architecture/internal/entity"
	"github.com/primestrider/golang-clean-architecture/internal/model"
	"github.com/primestrider/golang-clean-architecture/internal/security"
	"github.com/primestrider/golang-clean-architecture/test/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegister_Success(t *testing.T) {
	app := testsupport.NewWebApp(t)

	resp, raw := app.Do(t, http.MethodPost, "/api/v1/users",
		model.RegisterUserRequest{Email: "Eko@Example.com", Password: "rahasia123", Name: "Eko"}, "")

	require.Equal(t, http.StatusCreated, resp.StatusCode, string(raw))
	user := testsupport.DecodeData[model.UserResponse](t, raw)
	assert.NotEmpty(t, user.ID)
	assert.Equal(t, "eko@example.com", user.Email)
	assert.Equal(t, "user", user.Role)
	assert.Equal(t, 1, user.Version)
	assert.NotContains(t, string(raw), "password")
}

func TestRegister_DuplicateEmailIsConflict(t *testing.T) {
	app := testsupport.NewWebApp(t)
	app.CreateUser(t, "eko@example.com", "rahasia123")

	resp, raw := app.Do(t, http.MethodPost, "/api/v1/users",
		model.RegisterUserRequest{Email: "EKO@example.com", Password: "rahasia123", Name: "Eko"}, "")

	assert.Equal(t, http.StatusConflict, resp.StatusCode, string(raw))
}

func TestRegister_InvalidIsProblemWithFields(t *testing.T) {
	app := testsupport.NewWebApp(t)

	resp, raw := app.Do(t, http.MethodPost, "/api/v1/users", model.RegisterUserRequest{Email: "bukan-email", Password: "short"}, "")

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "application/problem+json", resp.Header.Get("Content-Type"))
	var fields []string
	for _, e := range testsupport.DecodeProblem(t, raw).Errors {
		fields = append(fields, e.Field)
	}
	assert.ElementsMatch(t, []string{"email", "password", "name"}, fields)
}

func TestCurrentUser_RequiresBearerToken(t *testing.T) {
	app := testsupport.NewWebApp(t)

	resp, _ := app.Do(t, http.MethodGet, "/api/v1/users/_current", nil, "not-a-jwt")

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestCurrentUser_Success(t *testing.T) {
	app := testsupport.NewWebApp(t)
	user := app.CreateUser(t, "eko@example.com", "rahasia123")

	resp, raw := app.Do(t, http.MethodGet, "/api/v1/users/_current", nil, app.TokenFor(t, user))

	require.Equal(t, http.StatusOK, resp.StatusCode, string(raw))
	assert.Equal(t, user.ID, testsupport.DecodeData[model.UserResponse](t, raw).ID)
}

func TestUpdateUser_ChangesName(t *testing.T) {
	app := testsupport.NewWebApp(t)
	user := app.CreateUser(t, "eko@example.com", "rahasia123")

	resp, raw := app.Do(t, http.MethodPatch, "/api/v1/users/_current", model.UpdateUserRequest{Name: "Eko Baru"}, app.TokenFor(t, user))

	require.Equal(t, http.StatusOK, resp.StatusCode, string(raw))
	assert.Equal(t, "Eko Baru", testsupport.DecodeData[model.UserResponse](t, raw).Name)
}

func TestUpdateUser_ChangesPasswordWithCurrentPassword(t *testing.T) {
	app := testsupport.NewWebApp(t)
	user := app.CreateUser(t, "eko@example.com", "rahasia123")

	resp, raw := app.Do(t, http.MethodPatch, "/api/v1/users/_current",
		model.UpdateUserRequest{CurrentPassword: "rahasia123", NewPassword: "passwordbaru"}, app.TokenFor(t, user))

	require.Equal(t, http.StatusOK, resp.StatusCode, string(raw))
	stored := new(entity.User)
	require.NoError(t, app.DB.First(stored, "id = ?", user.ID).Error)
	assert.True(t, security.ComparePassword(stored.Password, "passwordbaru"))
}
