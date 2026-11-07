//go:build integration

package test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/primestrider/golang-clean-architecture/internal/model"
	"github.com/primestrider/golang-clean-architecture/test/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func jsonUnmarshal(raw []byte, v any) error { return json.Unmarshal(raw, v) }

func TestCreateAddress_Success(t *testing.T) {
	app, userID, token := loggedIn(t)
	contact := app.CreateContact(t, userID, "Budi")

	resp, raw := app.Do(t, http.MethodPost, "/api/v1/contacts/"+contact.ID+"/addresses",
		model.CreateAddressRequest{Street: "Jl. Merdeka", City: "Bandung", Country: "Indonesia"}, token)

	require.Equal(t, http.StatusOK, resp.StatusCode, string(raw))
	assert.Equal(t, "Bandung", testsupport.DecodeData[model.AddressResponse](t, raw).City)
}

func TestCreateAddress_OnOtherUsersContactIsNotFound(t *testing.T) {
	app, _, token := loggedIn(t)
	other := app.CreateUser(t, "other@example.com", "rahasia123")
	contact := app.CreateContact(t, other.ID, "Rahasia")

	resp, _ := app.Do(t, http.MethodPost, "/api/v1/contacts/"+contact.ID+"/addresses",
		model.CreateAddressRequest{City: "Bandung"}, token)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestListAddresses(t *testing.T) {
	app, userID, token := loggedIn(t)
	contact := app.CreateContact(t, userID, "Budi")
	app.CreateAddress(t, contact.ID, "Jakarta")
	app.CreateAddress(t, contact.ID, "Bandung")

	resp, raw := app.Do(t, http.MethodGet, "/api/v1/contacts/"+contact.ID+"/addresses", nil, token)

	require.Equal(t, http.StatusOK, resp.StatusCode, string(raw))
	assert.Len(t, testsupport.DecodeData[[]model.AddressResponse](t, raw), 2)
}

func TestGetUpdateDeleteAddress(t *testing.T) {
	app, userID, token := loggedIn(t)
	contact := app.CreateContact(t, userID, "Budi")
	address := app.CreateAddress(t, contact.ID, "Jakarta")
	path := "/api/v1/contacts/" + contact.ID + "/addresses/" + address.ID

	resp, raw := app.Do(t, http.MethodGet, path, nil, token)
	require.Equal(t, http.StatusOK, resp.StatusCode, string(raw))

	resp, raw = app.Do(t, http.MethodPut, path, model.UpdateAddressRequest{City: "Surabaya", Version: 1}, token)
	require.Equal(t, http.StatusOK, resp.StatusCode, string(raw))
	assert.Equal(t, "Surabaya", testsupport.DecodeData[model.AddressResponse](t, raw).City)

	resp, _ = app.Do(t, http.MethodDelete, path, nil, token)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	resp, _ = app.Do(t, http.MethodGet, path, nil, token)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
