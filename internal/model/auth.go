package model

// Auth adalah identitas user yang sedang login, hasil verifikasi access
// token oleh middleware autentikasi. Controller membacanya untuk mengisi
// field UserID pada request, dan middleware otorisasi memakai Role.
type Auth struct {
	UserID string
	Role   string
}
