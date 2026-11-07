//go:build integration

package test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/primestrider/golang-clean-architecture/internal/entity"
	"github.com/primestrider/golang-clean-architecture/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateContact_WritesOutboxRow(t *testing.T) {
	app, userID, token := loggedIn(t)

	resp, raw := app.Do(t, http.MethodPost, "/api/v1/contacts", model.CreateContactRequest{FirstName: "Budi"}, token)
	require.Equal(t, http.StatusOK, resp.StatusCode, string(raw))
	var created model.WebResponse[model.ContactResponse]
	require.NoError(t, json.Unmarshal(raw, &created))

	var rows []entity.OutboxEvent
	require.NoError(t, app.DB.Where("event_type = ?", "contact.created").Find(&rows).Error)
	require.Len(t, rows, 1)
	row := rows[0]
	assert.Equal(t, "contacts", row.Topic)
	assert.Equal(t, created.Data.ID, row.MsgKey)
	assert.Equal(t, created.Data.ID, row.AggregateID)
	assert.Equal(t, entity.OutboxPending, row.Status)

	var event model.Event
	require.NoError(t, json.Unmarshal(row.Payload, &event))
	assert.Equal(t, row.ID, event.EventID)
	assert.Equal(t, userID, event.ActorID)
}

func TestCreateContact_ValidationFailureWritesNoOutboxRow(t *testing.T) {
	app, _, token := loggedIn(t)

	resp, _ := app.Do(t, http.MethodPost, "/api/v1/contacts", model.CreateContactRequest{}, token)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var count int64
	require.NoError(t, app.DB.Model(&entity.OutboxEvent{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestRegister_OutboxPayloadHasNoPassword(t *testing.T) {
	app, _, _ := loggedIn(t)

	resp, _ := app.Do(t, http.MethodPost, "/api/v1/users", model.RegisterUserRequest{Email: "new@example.com", Password: "rahasia123", Name: "New"}, "")
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var row entity.OutboxEvent
	require.NoError(t, app.DB.Where("event_type = ?", "user.registered").First(&row).Error)
	assert.NotContains(t, string(row.Payload), "password")
	assert.NotContains(t, string(row.Payload), "rahasia123")
	assert.Contains(t, string(row.Payload), "new@example.com")
}
