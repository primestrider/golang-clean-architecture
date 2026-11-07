package model

// Auth adalah identitas user yang sedang login, hasil verifikasi token oleh
// middleware autentikasi. Controller membacanya untuk mengisi field UserID
// pada request.
type Auth struct {
	// ID adalah id user yang login.
	ID string
}
