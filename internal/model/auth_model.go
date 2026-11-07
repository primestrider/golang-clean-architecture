package model

// LoginRequest adalah kredensial login. UserAgent dan IP diisi controller
// dari request HTTP dan disimpan bersama refresh token untuk keperluan audit.
type LoginRequest struct {
	Email     string `json:"email" validate:"required,email,max=255"`
	Password  string `json:"password" validate:"required,max=72"`
	UserAgent string `json:"-"`
	IP        string `json:"-"`
}

// RefreshRequest meminta pasangan token baru dengan refresh token.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required,max=128"`
	UserAgent    string `json:"-"`
	IP           string `json:"-"`
}

// LogoutRequest mengakhiri sesi milik refresh token tersebut.
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required,max=128"`
}

// TokenResponse adalah pasangan token hasil login atau refresh.
//
// AccessToken dikirim di header "Authorization: Bearer ..." pada setiap
// request dan berumur pendek. RefreshToken hanya dipakai untuk meminta
// pasangan token baru; simpan di tempat aman di sisi client.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	// ExpiresIn adalah sisa umur access token dalam detik.
	ExpiresIn int `json:"expires_in"`
}
