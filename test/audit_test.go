//go:build integration

package test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/primestrider/golang-clean-architecture/internal/entity"
	"github.com/primestrider/golang-clean-architecture/internal/model"
	"github.com/primestrider/golang-clean-architecture/internal/repository"
	"github.com/primestrider/golang-clean-architecture/internal/usecase"
	"github.com/primestrider/golang-clean-architecture/test/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func auditLogs(t *testing.T, app *testsupport.TestApp, action string) []entity.AuditLog {
	t.Helper()
	var logs []entity.AuditLog
	require.NoError(t, app.DB.Where("action = ?", action).Find(&logs).Error)
	return logs
}

func TestAuditRecorder_RollsBackWithTransaction(t *testing.T) {
	app := testsupport.NewWebApp(t)
	recorder := usecase.NewAuditRecorder(repository.NewAuditRepository(app.DB))
	tx := repository.NewTransactor(app.DB)

	err := tx.WithinTransaction(context.Background(), func(ctx context.Context) error {
		require.NoError(t, recorder.Record(ctx, "contact.updated", "contact", "c1", nil, nil))
		return errors.New("business failure after audit write")
	})

	require.Error(t, err)
	assert.Empty(t, auditLogs(t, app, "contact.updated"))
}

func TestUpdateContact_WritesAudit(t *testing.T) {
	app, userID, token := loggedIn(t)
	contact := app.CreateContact(t, userID, "Budi")

	resp, raw := app.Do(t, http.MethodPut, "/api/v1/contacts/"+contact.ID, model.UpdateContactRequest{FirstName: "Budi Baru", Email: contact.Email, Phone: contact.Phone, Version: 1}, token)
	require.Equal(t, http.StatusOK, resp.StatusCode, string(raw))

	logs := auditLogs(t, app, "contact.updated")
	require.Len(t, logs, 1)
	assert.Equal(t, userID, *logs[0].ActorID)
	assert.Equal(t, contact.ID, logs[0].EntityID)
	assert.NotNil(t, logs[0].RequestID)
	assert.JSONEq(t, `{"first_name":["Budi","Budi Baru"],"version":[1,2]}`, string(logs[0].Changes))
}

func TestDeleteContact_WritesAudit(t *testing.T) {
	app, userID, token := loggedIn(t)
	contact := app.CreateContact(t, userID, "Budi")

	resp, _ := app.Do(t, http.MethodDelete, "/api/v1/contacts/"+contact.ID, nil, token)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	assert.Len(t, auditLogs(t, app, "contact.deleted"), 1)
}

func TestRegister_WritesAuditWithoutPassword(t *testing.T) {
	app := testsupport.NewWebApp(t)

	resp, _ := app.Do(t, http.MethodPost, "/api/v1/users", model.RegisterUserRequest{Email: "eko@example.com", Password: "rahasia123", Name: "Eko"}, "")
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	logs := auditLogs(t, app, "user.registered")
	require.Len(t, logs, 1)
	assert.Nil(t, logs[0].ActorID)
	assert.NotContains(t, string(logs[0].Changes), "password")
	assert.NotContains(t, string(logs[0].Changes), "rahasia123")
}

func TestPasswordChange_WritesAuditWithoutSecrets(t *testing.T) {
	app := testsupport.NewWebApp(t)
	user := app.CreateUser(t, "eko@example.com", "rahasia123")

	resp, _ := app.Do(t, http.MethodPatch, "/api/v1/users/_current",
		model.UpdateUserRequest{CurrentPassword: "rahasia123", NewPassword: "passwordbaru", Version: 1}, app.TokenFor(t, user))
	require.Equal(t, http.StatusOK, resp.StatusCode)

	logs := auditLogs(t, app, "user.password_changed")
	require.Len(t, logs, 1)
	assert.NotContains(t, string(logs[0].Changes), "passwordbaru")
	assert.NotContains(t, string(logs[0].Changes), "$2a$")
}
