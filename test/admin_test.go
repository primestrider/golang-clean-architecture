//go:build integration

package test

import (
	"net/http"
	"testing"

	"github.com/primestrider/golang-clean-architecture/internal/entity"
	"github.com/primestrider/golang-clean-architecture/internal/model"
	"github.com/primestrider/golang-clean-architecture/test/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createAdmin(t *testing.T, app *testsupport.TestApp) *entity.User {
	admin := app.CreateUser(t, "admin@example.com", "rahasia123")
	require.NoError(t, app.DB.Model(admin).Update("role", entity.RoleAdmin).Error)
	admin.Role = entity.RoleAdmin
	return admin
}

func TestAdminUsers_ForbiddenForUserRole(t *testing.T) {
	app := testsupport.NewWebApp(t)
	user := app.CreateUser(t, "eko@example.com", "rahasia123")

	resp, _ := app.Do(t, http.MethodGet, "/api/v1/admin/users", nil, app.TokenFor(t, user))

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestAdminUsers_OKForAdmin(t *testing.T) {
	app := testsupport.NewWebApp(t)
	admin := createAdmin(t, app)
	app.CreateUser(t, "eko@example.com", "rahasia123")

	resp, raw := app.Do(t, http.MethodGet, "/api/v1/admin/users?page=1&size=10", nil, app.TokenFor(t, admin))

	require.Equal(t, http.StatusOK, resp.StatusCode, string(raw))
	assert.Len(t, testsupport.DecodeData[[]model.UserResponse](t, raw), 2)
}

func TestAdminChangeRole(t *testing.T) {
	app := testsupport.NewWebApp(t)
	admin := createAdmin(t, app)
	user := app.CreateUser(t, "eko@example.com", "rahasia123")

	resp, raw := app.Do(t, http.MethodPatch, "/api/v1/admin/users/"+user.ID+"/role",
		model.ChangeRoleRequest{Role: "admin"}, app.TokenFor(t, admin))

	require.Equal(t, http.StatusOK, resp.StatusCode, string(raw))
	assert.Equal(t, "admin", testsupport.DecodeData[model.UserResponse](t, raw).Role)
}
