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

func TestUpdateContact_VersionIncrements(t *testing.T) {
	app, userID, token := loggedIn(t)
	contact := app.CreateContact(t, userID, "Budi")

	resp, raw := app.Do(t, http.MethodPut, "/api/v1/contacts/"+contact.ID,
		model.UpdateContactRequest{FirstName: "Budi Baru", Version: 1}, token)

	require.Equal(t, http.StatusOK, resp.StatusCode, string(raw))
	assert.Equal(t, 2, testsupport.DecodeData[model.ContactResponse](t, raw).Version)
}

func TestUpdateContact_StaleVersion409(t *testing.T) {
	app, userID, token := loggedIn(t)
	contact := app.CreateContact(t, userID, "Budi")
	resp, _ := app.Do(t, http.MethodPut, "/api/v1/contacts/"+contact.ID, model.UpdateContactRequest{FirstName: "A", Version: 1}, token)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Request kedua masih membawa version 1 (data yang sudah usang).
	resp, raw := app.Do(t, http.MethodPut, "/api/v1/contacts/"+contact.ID, model.UpdateContactRequest{FirstName: "B", Version: 1}, token)

	assert.Equal(t, http.StatusConflict, resp.StatusCode, string(raw))
}

func TestUpdateContact_MissingVersionIsValidationError(t *testing.T) {
	app, userID, token := loggedIn(t)
	contact := app.CreateContact(t, userID, "Budi")

	resp, raw := app.Do(t, http.MethodPut, "/api/v1/contacts/"+contact.ID, model.UpdateContactRequest{FirstName: "A"}, token)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "version", testsupport.DecodeProblem(t, raw).Errors[0].Field)
}

func TestDeletedContact_NotListedAndGet404(t *testing.T) {
	app, userID, token := loggedIn(t)
	contact := app.CreateContact(t, userID, "Budi")
	app.CreateContact(t, userID, "Andi")

	resp, _ := app.Do(t, http.MethodDelete, "/api/v1/contacts/"+contact.ID, nil, token)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	resp, _ = app.Do(t, http.MethodGet, "/api/v1/contacts/"+contact.ID, nil, token)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	_, raw := app.Do(t, http.MethodGet, "/api/v1/contacts", nil, token)
	assert.Len(t, testsupport.DecodeData[[]model.ContactResponse](t, raw), 1)

	// Datanya masih ada di database (soft delete), hanya ditandai terhapus.
	var deletedAt int64
	require.NoError(t, app.DB.Raw("SELECT deleted_at FROM contacts WHERE id = ?", contact.ID).Scan(&deletedAt).Error)
	assert.NotZero(t, deletedAt)
}

func TestDeleteContact_SoftDeletesAddresses(t *testing.T) {
	app, userID, token := loggedIn(t)
	contact := app.CreateContact(t, userID, "Budi")
	app.CreateAddress(t, contact.ID, "Jakarta")
	app.CreateAddress(t, contact.ID, "Bandung")

	resp, _ := app.Do(t, http.MethodDelete, "/api/v1/contacts/"+contact.ID, nil, token)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var active int64
	require.NoError(t, app.DB.Raw("SELECT COUNT(*) FROM addresses WHERE contact_id = ? AND deleted_at = 0", contact.ID).Scan(&active).Error)
	assert.Zero(t, active)
}

// TestUpdate_StaleVersionOnSoftDeleted404: data yang sudah dihapus harus
// dilaporkan "tidak ditemukan", bukan "konflik versi".
func TestUpdate_StaleVersionOnSoftDeleted404(t *testing.T) {
	app, userID, token := loggedIn(t)
	contact := app.CreateContact(t, userID, "Budi")
	resp, _ := app.Do(t, http.MethodDelete, "/api/v1/contacts/"+contact.ID, nil, token)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	resp, raw := app.Do(t, http.MethodPut, "/api/v1/contacts/"+contact.ID, model.UpdateContactRequest{FirstName: "X", Version: 7}, token)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode, string(raw))
}

func TestUpdateAddress_StaleVersion409(t *testing.T) {
	app, userID, token := loggedIn(t)
	contact := app.CreateContact(t, userID, "Budi")
	address := app.CreateAddress(t, contact.ID, "Jakarta")
	path := "/api/v1/contacts/" + contact.ID + "/addresses/" + address.ID

	resp, _ := app.Do(t, http.MethodPut, path, model.UpdateAddressRequest{City: "Bogor", Version: 1}, token)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp, _ = app.Do(t, http.MethodPut, path, model.UpdateAddressRequest{City: "Depok", Version: 1}, token)

	assert.Equal(t, http.StatusConflict, resp.StatusCode)
}

func TestUpdateUser_StaleVersion409(t *testing.T) {
	app := testsupport.NewWebApp(t)
	user := app.CreateUser(t, "eko@example.com", "rahasia123")
	token := app.TokenFor(t, user)
	resp, _ := app.Do(t, http.MethodPatch, "/api/v1/users/_current", model.UpdateUserRequest{Name: "A", Version: 1}, token)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	resp, _ = app.Do(t, http.MethodPatch, "/api/v1/users/_current", model.UpdateUserRequest{Name: "B", Version: 1}, token)

	assert.Equal(t, http.StatusConflict, resp.StatusCode)
}

func TestRegister_EmailReusableAfterSoftDelete(t *testing.T) {
	app := testsupport.NewWebApp(t)
	user := app.CreateUser(t, "eko@example.com", "rahasia123")
	require.NoError(t, app.DB.Delete(&entity.User{}, "id = ?", user.ID).Error)

	resp, raw := app.Do(t, http.MethodPost, "/api/v1/users",
		model.RegisterUserRequest{Email: "eko@example.com", Password: "rahasia123", Name: "Eko"}, "")

	assert.Equal(t, http.StatusCreated, resp.StatusCode, string(raw))
}
