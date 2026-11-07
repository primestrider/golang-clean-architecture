package config

import (
	"log/slog"

	"github.com/primestrider/golang-clean-architecture/internal/gateway/email"
	"github.com/primestrider/golang-clean-architecture/internal/usecase"
)

// NewMailer membuat pengirim email: SMTP bila email aktif, atau pengirim
// log-only bila dimatikan.
func NewMailer(cfg *Config, log *slog.Logger) (usecase.Mailer, error) {
	if !cfg.Email.Enabled {
		return email.NewLogMailer(log), nil
	}
	return email.NewSMTPMailer(email.Options{
		Host:     cfg.Email.Host,
		Port:     cfg.Email.Port,
		Username: cfg.Email.Username,
		Password: cfg.Email.Password,
		From:     cfg.Email.From,
		TLS:      cfg.Email.TLS,
	})
}
