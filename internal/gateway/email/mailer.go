// Package email berisi implementasi port usecase.Mailer: pengirim SMTP
// (bisa diarahkan ke Amazon SES, SendGrid, Mailgun, atau Mailpit saat
// development) dan pengirim log-only saat email dimatikan.
package email

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/primestrider/golang-clean-architecture/internal/model"
	"github.com/wneessen/go-mail"
)

// Options berisi pengaturan server SMTP.
type Options struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	// TLS mewajibkan STARTTLS. Matikan hanya untuk server lokal seperti Mailpit.
	TLS bool
}

// SMTPMailer mengirim email lewat SMTP.
type SMTPMailer struct {
	client *mail.Client
	from   string
}

// NewSMTPMailer membuat SMTPMailer. Koneksi dibuka saat mengirim, bukan di sini.
func NewSMTPMailer(opts Options) (*SMTPMailer, error) {
	clientOpts := []mail.Option{mail.WithPort(opts.Port)}
	if opts.TLS {
		clientOpts = append(clientOpts, mail.WithTLSPolicy(mail.TLSMandatory))
	} else {
		clientOpts = append(clientOpts, mail.WithTLSPolicy(mail.NoTLS))
	}
	if opts.Username != "" {
		clientOpts = append(clientOpts,
			mail.WithSMTPAuth(mail.SMTPAuthPlain),
			mail.WithUsername(opts.Username),
			mail.WithPassword(opts.Password),
		)
	}
	client, err := mail.NewClient(opts.Host, clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("create smtp client: %w", err)
	}
	return &SMTPMailer{client: client, from: opts.From}, nil
}

// Send mengirim satu email berisi versi teks dan HTML.
func (m *SMTPMailer) Send(ctx context.Context, msg model.EmailMessage) error {
	message := mail.NewMsg()
	if err := message.From(m.from); err != nil {
		return fmt.Errorf("invalid from address: %w", err)
	}
	if err := message.To(msg.To); err != nil {
		return fmt.Errorf("invalid recipient: %w", err)
	}
	message.Subject(msg.Subject)
	message.SetBodyString(mail.TypeTextPlain, msg.TextBody)
	message.AddAlternativeString(mail.TypeTextHTML, msg.HTMLBody)
	return m.client.DialAndSendWithContext(ctx, message)
}

// LogMailer hanya mencatat email ke log tanpa mengirimnya. Dipakai saat
// email dimatikan, supaya alur yang mengirim email tetap bisa diuji.
type LogMailer struct {
	log *slog.Logger
}

// NewLogMailer membuat LogMailer.
func NewLogMailer(log *slog.Logger) *LogMailer {
	return &LogMailer{log: log}
}

// Send mencatat penerima dan subjek email (tanpa isi) ke log.
func (m *LogMailer) Send(ctx context.Context, msg model.EmailMessage) error {
	m.log.InfoContext(ctx, "email not sent (email disabled)", "subject", msg.Subject)
	return nil
}
