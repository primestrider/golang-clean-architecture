//go:build integration

package test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/primestrider/golang-clean-architecture/internal/model"
	"github.com/primestrider/golang-clean-architecture/test/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// loggedIn menyiapkan aplikasi dengan satu user yang sudah login.
func loggedIn(t *testing.T) (*testsupport.TestApp, string, string) {
	app := testsupport.NewWebApp(t)
	user := app.CreateUser(t, "eko@example.com", "rahasia123")
	return app, user.ID, app.TokenFor(t, user)
}

func TestCreateContact_Success(t *testing.T) {
	app, _, token := loggedIn(t)

	resp, raw := app.Do(t, http.MethodPost, "/api/v1/contacts",
		model.CreateContactRequest{FirstName: "Budi", LastName: "Santoso", Email: "budi@example.com", Phone: "0811"}, token)

	require.Equal(t, http.StatusOK, resp.StatusCode, string(raw))
	contact := testsupport.DecodeData[model.ContactResponse](t, raw)
	assert.Equal(t, "Budi", contact.FirstName)
	assert.NotEmpty(t, contact.ID)
	assert.NotZero(t, contact.CreatedAt)
}

func TestCreateContact_InvalidEmail(t *testing.T) {
	app, _, token := loggedIn(t)

	resp, raw := app.Do(t, http.MethodPost, "/api/v1/contacts",
		model.CreateContactRequest{FirstName: "Budi", Email: "bukan-email"}, token)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "email", testsupport.DecodeProblem(t, raw).Errors[0].Field)
}

func TestGetContact_Success(t *testing.T) {
	app, userID, token := loggedIn(t)
	contact := app.CreateContact(t, userID, "Budi")

	resp, raw := app.Do(t, http.MethodGet, "/api/v1/contacts/"+contact.ID, nil, token)

	require.Equal(t, http.StatusOK, resp.StatusCode, string(raw))
	assert.Equal(t, contact.ID, testsupport.DecodeData[model.ContactResponse](t, raw).ID)
}

func TestGetContact_OtherUsersContactIsNotFound(t *testing.T) {
	app, _, token := loggedIn(t)
	other := app.CreateUser(t, "other@example.com", "rahasia123")
	contact := app.CreateContact(t, other.ID, "Rahasia")

	resp, _ := app.Do(t, http.MethodGet, "/api/v1/contacts/"+contact.ID, nil, token)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestUpdateContact_Success(t *testing.T) {
	app, userID, token := loggedIn(t)
	contact := app.CreateContact(t, userID, "Budi")

	resp, raw := app.Do(t, http.MethodPut, "/api/v1/contacts/"+contact.ID,
		model.UpdateContactRequest{FirstName: "Budi Baru"}, token)

	require.Equal(t, http.StatusOK, resp.StatusCode, string(raw))
	assert.Equal(t, "Budi Baru", testsupport.DecodeData[model.ContactResponse](t, raw).FirstName)
}

func TestDeleteContact_Success(t *testing.T) {
	app, userID, token := loggedIn(t)
	contact := app.CreateContact(t, userID, "Budi")

	resp, _ := app.Do(t, http.MethodDelete, "/api/v1/contacts/"+contact.ID, nil, token)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	resp, _ = app.Do(t, http.MethodGet, "/api/v1/contacts/"+contact.ID, nil, token)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestSearchContact_Paging(t *testing.T) {
	app, userID, token := loggedIn(t)
	for i := range 15 {
		app.CreateContact(t, userID, fmt.Sprintf("Kontak%02d", i))
	}

	resp, raw := app.Do(t, http.MethodGet, "/api/v1/contacts?page=2&size=10", nil, token)

	require.Equal(t, http.StatusOK, resp.StatusCode, string(raw))
	var body model.WebResponse[[]model.ContactResponse]
	require.NoError(t, jsonUnmarshal(raw, &body))
	assert.Len(t, body.Data, 5)
	assert.Equal(t, &model.PageMetadata{Page: 2, Size: 10, TotalItem: 15, TotalPage: 2}, body.Paging)
}

func TestSearchContact_FilterByName(t *testing.T) {
	app, userID, token := loggedIn(t)
	app.CreateContact(t, userID, "Budi")
	app.CreateContact(t, userID, "Andi")

	resp, raw := app.Do(t, http.MethodGet, "/api/v1/contacts?name=bud", nil, token)

	require.Equal(t, http.StatusOK, resp.StatusCode, string(raw))
	contacts := testsupport.DecodeData[[]model.ContactResponse](t, raw)
	require.Len(t, contacts, 1)
	assert.Equal(t, "Budi", contacts[0].FirstName)
}
