//go:build integration

package test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/primestrider/golang-clean-architecture/internal/delivery/messaging"
	"github.com/primestrider/golang-clean-architecture/internal/entity"
	"github.com/primestrider/golang-clean-architecture/internal/gateway/email"
	"github.com/primestrider/golang-clean-architecture/internal/model"
	"github.com/primestrider/golang-clean-architecture/internal/outbox"
	"github.com/primestrider/golang-clean-architecture/internal/repository"
	"github.com/primestrider/golang-clean-architecture/internal/usecase"
	"github.com/primestrider/golang-clean-architecture/test/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWelcomeEmail_SentOnceViaLocalDispatcher: tanpa Kafka, registrasi
// memicu email selamat datang lewat outbox + dispatcher lokal, dan event
// yang diproses ulang tidak mengirim email kedua.
func TestWelcomeEmail_SentOnceViaLocalDispatcher(t *testing.T) {
	mailpit := testsupport.StartMailpit(t)
	app := testsupport.NewWebApp(t)
	mailer, err := email.NewSMTPMailer(email.Options{Host: mailpit.Host, Port: mailpit.SMTPPort, From: "no-reply@example.com"})
	require.NoError(t, err)
	registry := messaging.NewRegistry(repository.NewTransactor(app.DB), repository.NewProcessedEventRepository(app.DB), discard())
	registry.Register("welcome-email", "user.registered", usecase.NewNotificationUseCase(discard(), mailer).SendWelcome)
	relay := outbox.NewRelay(repository.NewOutboxRepository(app.DB), messaging.NewLocalDispatcher(registry),
		outbox.NewMySQLLocker(app.DB, "test-email"), outbox.Options{BatchSize: 100, MaxAttempts: 3}, discard())

	resp, _ := app.Do(t, http.MethodPost, "/api/v1/users", model.RegisterUserRequest{Email: "new@example.com", Password: "rahasia123", Name: "New"}, "")
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	published, err := relay.ProcessBatch(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, published)
	assert.Equal(t, 1, mailpit.MessageCount(t))

	// Simulasi pengiriman ulang (at-least-once): event yang sama diproses lagi.
	var row entity.OutboxEvent
	require.NoError(t, app.DB.Where("event_type = ?", "user.registered").First(&row).Error)
	var e model.Event
	require.NoError(t, json.Unmarshal(row.Payload, &e))
	require.NoError(t, registry.Handle(context.Background(), e))

	assert.Equal(t, 1, mailpit.MessageCount(t), "duplicate delivery must not send a second email")
}
