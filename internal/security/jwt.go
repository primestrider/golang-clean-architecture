package security

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/primestrider/golang-clean-architecture/internal/apperror"
)

// Claims adalah isi access token. UserID disimpan di claim standar "sub"
// (subject) dan Role di claim kustom "role".
type Claims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
	// UserID diisi dari Subject setelah token diverifikasi; tidak ikut
	// diserialisasi sebagai claim terpisah.
	UserID string `json:"-"`
}

// TokenIssuer menerbitkan dan memverifikasi access token JWT bertanda
// tangan HMAC-SHA256 (HS256).
//
// HS256 memakai satu secret yang sama untuk menandatangani dan memverifikasi.
// Cocok bila hanya layanan ini yang memverifikasi token. Bila token harus
// diverifikasi layanan lain, pindah ke algoritma asimetris (RS256/EdDSA)
// supaya layanan lain cukup memegang public key.
type TokenIssuer struct {
	secret   []byte
	issuer   string
	audience string
	ttl      time.Duration
	now      func() time.Time
}

// NewTokenIssuer membuat TokenIssuer. now disuntikkan supaya kedaluwarsa
// token bisa diuji tanpa menunggu waktu berlalu.
func NewTokenIssuer(secret []byte, issuer, audience string, ttl time.Duration, now func() time.Time) *TokenIssuer {
	return &TokenIssuer{secret: secret, issuer: issuer, audience: audience, ttl: ttl, now: now}
}

// Issue menerbitkan access token untuk user dengan role tertentu.
// Setiap token punya jti (id unik) supaya bisa dilacak di log bila perlu.
func (t *TokenIssuer) Issue(userID, role string) (string, time.Time, error) {
	now := t.now()
	expiresAt := now.Add(t.ttl)
	claims := Claims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    t.issuer,
			Audience:  jwt.ClaimStrings{t.audience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			ID:        uuid.NewString(),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(t.secret)
	if err != nil {
		return "", time.Time{}, apperror.Internal(err)
	}
	return token, expiresAt, nil
}

// Parse memverifikasi token dan mengembalikan claims-nya.
//
// Verifikasi yang dilakukan: algoritma wajib HS256 (mencegah serangan
// "alg: none" dan kebingungan algoritma), tanda tangan, issuer, audience,
// dan masa berlaku. Kegagalan apa pun dilaporkan sebagai UNAUTHORIZED
// dengan pesan generik, tanpa membocorkan alasan spesifiknya.
func (t *TokenIssuer) Parse(token string) (*Claims, error) {
	claims := new(Claims)
	_, err := jwt.ParseWithClaims(token, claims,
		func(*jwt.Token) (any, error) { return t.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(t.issuer),
		jwt.WithAudience(t.audience),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(t.now),
	)
	if err != nil || claims.Subject == "" {
		return nil, apperror.Wrap(apperror.CodeUnauthorized, "invalid or expired token", err)
	}
	claims.UserID = claims.Subject
	return claims, nil
}
