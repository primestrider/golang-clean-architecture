package model

// EmailMessage adalah email yang akan dikirim. HTMLBody dan TextBody berisi
// isi yang sama; TextBody untuk aplikasi email yang tidak menampilkan HTML.
type EmailMessage struct {
	To       string
	Subject  string
	HTMLBody string
	TextBody string
}
