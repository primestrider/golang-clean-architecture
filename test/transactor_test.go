//go:build integration

package test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/primestrider/golang-clean-architecture/internal/entity"
	"github.com/primestrider/golang-clean-architecture/internal/repository"
	"github.com/primestrider/golang-clean-architecture/test/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransactor_RollsBackOnError(t *testing.T) {
	app := testsupport.NewWebApp(t)
	user := app.CreateUser(t, "eko@example.com", "rahasia123")
	tx := repository.NewTransactor(app.DB)
	contacts := repository.NewContactRepository(app.DB)
	contact := &entity.Contact{ID: uuid.NewString(), UserID: user.ID, FirstName: "Budi"}
	boom := errors.New("boom")

	err := tx.WithinTransaction(context.Background(), func(ctx context.Context) error {
		require.NoError(t, contacts.Create(ctx, contact))
		return boom
	})

	assert.ErrorIs(t, err, boom)
	var count int64
	require.NoError(t, app.DB.Model(&entity.Contact{}).Where("id = ?", contact.ID).Count(&count).Error)
	assert.Zero(t, count, "contact must be rolled back")
}

func TestTransactor_CommitsOnSuccess(t *testing.T) {
	app := testsupport.NewWebApp(t)
	user := app.CreateUser(t, "eko@example.com", "rahasia123")
	tx := repository.NewTransactor(app.DB)
	contacts := repository.NewContactRepository(app.DB)
	contact := &entity.Contact{ID: uuid.NewString(), UserID: user.ID, FirstName: "Budi"}

	err := tx.WithinTransaction(context.Background(), func(ctx context.Context) error {
		return contacts.Create(ctx, contact)
	})

	require.NoError(t, err)
	_, err = contacts.FindByID(context.Background(), contact.ID)
	assert.NoError(t, err)
}
