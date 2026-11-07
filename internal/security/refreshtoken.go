package security

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// refreshTokenBytes adalah jumlah byte acak refresh token (256 bit),
// cukup besar sehingga tidak mungkin ditebak.
const refreshTokenBytes = 32

// NewRefreshToken membuat refresh token acak.
//
// plain dikirim ke client sekali saja. Yang disimpan di database hanya hash,
// sehingga kebocoran database tidak langsung memberi penyerang token yang
// bisa dipakai.
//
// Refresh token tidak di-hash dengan bcrypt karena token ini sudah acak
// penuh (bukan password pilihan manusia). SHA-256 cukup, dan memungkinkan
// pencarian langsung berdasarkan hash lewat index database.
func NewRefreshToken() (plain, hash string, err error) {
	buf := make([]byte, refreshTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	plain = base64.RawURLEncoding.EncodeToString(buf)
	return plain, HashRefreshToken(plain), nil
}

// HashRefreshToken menghitung hash SHA-256 (hex) dari refresh token.
func HashRefreshToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}
