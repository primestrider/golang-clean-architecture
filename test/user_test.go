//go:build integration

package test

import (
	"net/http"
	"testing"

	"github.com/primestrider/golang-clean-architecture/internal/model"
	"github.com/primestrider/golang-clean-architecture/test/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegister_Success(t *testing.T) {
	app := testsupport.NewWebApp(t)

	resp, raw := app.Do(t, http.MethodPost, "/api/v1/users",
		model.RegisterUserRequest{ID: "eko", Password: "rahasia123", Name: "Eko"}, "")

	require.Equal(t, http.StatusOK, resp.StatusCode, string(raw))
	user := testsupport.DecodeData[model.UserResponse](t, raw)
	assert.Equal(t, "eko", user.ID)
	assert.Equal(t, "Eko", user.Name)
	assert.NotContains(t, string(raw), "password")
}

func TestRegister_DuplicateIsConflict(t *testing.T) {
	app := testsupport.NewWebApp(t)
	app.CreateUser(t, "eko", "rahasia123")

	resp, raw := app.Do(t, http.MethodPost, "/api/v1/users",
		model.RegisterUserRequest{ID: "eko", Password: "rahasia123", Name: "Eko"}, "")

	assert.Equal(t, http.StatusConflict, resp.StatusCode, string(raw))
}

func TestRegister_InvalidIsProblemWithFields(t *testing.T) {
	app := testsupport.NewWebApp(t)

	resp, raw := app.Do(t, http.MethodPost, "/api/v1/users", model.RegisterUserRequest{}, "")

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "application/problem+json", resp.Header.Get("Content-Type"))
	problem := testsupport.DecodeProblem(t, raw)
	assert.NotEmpty(t, problem.Errors)
}

func TestLogin_WrongPasswordIsUnauthorized(t *testing.T) {
	app := testsupport.NewWebApp(t)
	app.CreateUser(t, "eko", "rahasia123")

	resp, _ := app.Do(t, http.MethodPost, "/api/v1/users/_login", model.LoginUserRequest{ID: "eko", Password: "salah"}, "")

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestCurrentUser_RequiresToken(t *testing.T) {
	app := testsupport.NewWebApp(t)

	resp, _ := app.Do(t, http.MethodGet, "/api/v1/users/_current", nil, "wrong-token")

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestCurrentUser_Success(t *testing.T) {
	app := testsupport.NewWebApp(t)
	app.CreateUser(t, "eko", "rahasia123")
	token := app.Login(t, "eko", "rahasia123")

	resp, raw := app.Do(t, http.MethodGet, "/api/v1/users/_current", nil, token)

	require.Equal(t, http.StatusOK, resp.StatusCode, string(raw))
	assert.Equal(t, "eko", testsupport.DecodeData[model.UserResponse](t, raw).ID)
}

func TestUpdateUser_ChangesName(t *testing.T) {
	app := testsupport.NewWebApp(t)
	app.CreateUser(t, "eko", "rahasia123")
	token := app.Login(t, "eko", "rahasia123")

	resp, raw := app.Do(t, http.MethodPatch, "/api/v1/users/_current", model.UpdateUserRequest{Name: "Eko Baru"}, token)

	require.Equal(t, http.StatusOK, resp.StatusCode, string(raw))
	assert.Equal(t, "Eko Baru", testsupport.DecodeData[model.UserResponse](t, raw).Name)
}

func TestLogout_InvalidatesToken(t *testing.T) {
	app := testsupport.NewWebApp(t)
	app.CreateUser(t, "eko", "rahasia123")
	token := app.Login(t, "eko", "rahasia123")

	resp, _ := app.Do(t, http.MethodDelete, "/api/v1/users", nil, token)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	resp, _ = app.Do(t, http.MethodGet, "/api/v1/users/_current", nil, token)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
