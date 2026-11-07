package usecase_test

import (
	"context"
	"testing"

	"github.com/primestrider/golang-clean-architecture/internal/model"
	"github.com/primestrider/golang-clean-architecture/internal/usecase"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeMailer mencatat email yang dikirim.
type fakeMailer struct{ sent []model.EmailMessage }

func (m *fakeMailer) Send(_ context.Context, msg model.EmailMessage) error {
	m.sent = append(m.sent, msg)
	return nil
}

func registeredEvent(t *testing.T, name string) model.Event {
	e, err := model.NewEvent("user.registered", "u1", "", model.UserResponse{ID: "u1", Email: "eko@example.com", Name: name})
	require.NoError(t, err)
	return e
}

func TestSendWelcome_SendsToRegisteredEmail(t *testing.T) {
	mailer := &fakeMailer{}
	uc := usecase.NewNotificationUseCase(discardLogger(), mailer)

	require.NoError(t, uc.SendWelcome(context.Background(), registeredEvent(t, "Eko")))

	require.Len(t, mailer.sent, 1)
	assert.Equal(t, "eko@example.com", mailer.sent[0].To)
	assert.Contains(t, mailer.sent[0].TextBody, "eko@example.com")
}

// TestSendWelcome_DoesNotIncludeUserSuppliedName: alamat email belum
// diverifikasi, jadi siapa pun bisa mendaftarkan email orang lain. Nama
// diisi bebas oleh pendaftar dan bisa berisi teks phishing ("klik
// http://..."), sehingga tidak boleh ikut dikirim ke alamat tersebut.
func TestSendWelcome_DoesNotIncludeUserSuppliedName(t *testing.T) {
	mailer := &fakeMailer{}
	uc := usecase.NewNotificationUseCase(discardLogger(), mailer)

	require.NoError(t, uc.SendWelcome(context.Background(), registeredEvent(t, "Verify at http://evil.example")))

	assert.NotContains(t, mailer.sent[0].HTMLBody, "evil.example")
	assert.NotContains(t, mailer.sent[0].TextBody, "evil.example")
}
