package config

import (
	"time"

	"github.com/primestrider/golang-clean-architecture/internal/security"
)

// NewTokenIssuer membuat penerbit access token JWT dari konfigurasi.
func NewTokenIssuer(cfg *Config) *security.TokenIssuer {
	return security.NewTokenIssuer([]byte(cfg.JWT.Secret), cfg.JWT.Issuer, cfg.JWT.Audience, cfg.JWT.AccessTTL, time.Now)
}
