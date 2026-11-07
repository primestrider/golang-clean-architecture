package usecase

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	htmltemplate "html/template"
	"log/slog"
	texttemplate "text/template"

	"github.com/primestrider/golang-clean-architecture/internal/model"
)

// templates berisi template email yang ikut dikompilasi ke dalam binary
// (go:embed), sehingga tidak perlu menyalin file template saat deploy.
//
//go:embed templates/*
var templates embed.FS

var (
	welcomeHTML = htmltemplate.Must(htmltemplate.ParseFS(templates, "templates/welcome.html"))
	welcomeText = texttemplate.Must(texttemplate.ParseFS(templates, "templates/welcome.txt"))
)

// NotificationUseCase mengirim notifikasi (email) sebagai reaksi atas event.
// Dijalankan di worker lewat handler event, bukan di request HTTP, sehingga
// registrasi tetap cepat dan tidak gagal walau server email sedang lambat.
type NotificationUseCase struct {
	log    *slog.Logger
	mailer Mailer
}

// NewNotificationUseCase membuat NotificationUseCase.
func NewNotificationUseCase(log *slog.Logger, mailer Mailer) *NotificationUseCase {
	return &NotificationUseCase{log: log, mailer: mailer}
}

// SendWelcome mengirim email selamat datang untuk event user.registered.
//
// Email sengaja tidak memuat nama yang diisi pendaftar. Alamat email belum
// diverifikasi, jadi siapa pun bisa mendaftarkan email orang lain; bila nama
// ikut dikirim, penyerang bisa mengisi nama dengan teks phishing yang lalu
// terkirim dari domain resmi aplikasi. Template HTML tetap memakai
// html/template yang meng-escape data sebagai lapisan pengaman tambahan.
func (c *NotificationUseCase) SendWelcome(ctx context.Context, e model.Event) error {
	var user model.UserResponse
	if err := json.Unmarshal(e.Data, &user); err != nil {
		return fmt.Errorf("decode user.registered payload: %w", err)
	}

	var html, text bytes.Buffer
	if err := welcomeHTML.Execute(&html, user); err != nil {
		return fmt.Errorf("render welcome html: %w", err)
	}
	if err := welcomeText.Execute(&text, user); err != nil {
		return fmt.Errorf("render welcome text: %w", err)
	}

	if err := c.mailer.Send(ctx, model.EmailMessage{
		To:       user.Email,
		Subject:  "Selamat datang!",
		HTMLBody: html.String(),
		TextBody: text.String(),
	}); err != nil {
		return fmt.Errorf("send welcome email: %w", err)
	}
	c.log.InfoContext(ctx, "welcome email sent", "user_id", user.ID)
	return nil
}
