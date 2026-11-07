package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeConfig menulis config.json minimal ke direktori sementara dan
// mengembalikan path direktorinya.
func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.json"), []byte(content), 0o600))
	return dir
}

const baseConfig = `{
  "app": {"name": "test", "env": "development"},
  "server": {"port": 3000, "request_timeout": "10s", "cors_origins": ["http://localhost:5173"]},
  "database": {"username": "root", "password": "", "host": "localhost", "port": 3306, "name": "db"},
  "jwt": {"secret": "dev-only-insecure-secret-change-me-32bytes", "issuer": "gca", "audience": "gca-api", "access_ttl": "15m", "refresh_ttl": "168h"},
  "docs": {"enabled": true}
}`

func TestLoad_EnvOverridesFile(t *testing.T) {
	dir := writeConfig(t, baseConfig)
	t.Setenv("APP_DATABASE_PASSWORD", "s3cret")

	cfg, err := Load(dir)

	require.NoError(t, err)
	assert.Equal(t, "s3cret", cfg.Database.Password)
	assert.Equal(t, 15*time.Minute, cfg.JWT.AccessTTL)
	assert.Equal(t, 10*time.Second, cfg.Server.RequestTimeout)
}

func TestLoad_EnvSetsKeyMissingFromFile(t *testing.T) {
	dir := writeConfig(t, baseConfig)
	t.Setenv("APP_REDIS_ADDR", "cache:6379")
	t.Setenv("APP_KAFKA_BROKERS", "k1:9092,k2:9092")

	cfg, err := Load(dir)

	require.NoError(t, err)
	assert.Equal(t, "cache:6379", cfg.Redis.Addr)
	assert.Equal(t, []string{"k1:9092", "k2:9092"}, cfg.Kafka.Brokers)
}

func validProductionConfig() *Config {
	return &Config{
		App:    AppConfig{Name: "test", Env: "production"},
		Server: ServerConfig{Port: 3000, CORSOrigins: []string{"https://app.example.com"}},
		JWT:    JWTConfig{Secret: strings.Repeat("x", 32), Issuer: "gca", Audience: "gca-api", AccessTTL: 15 * time.Minute, RefreshTTL: 168 * time.Hour},
		Docs:   DocsConfig{Enabled: false},
	}
}

func TestValidate_ProductionRejectsShortJWTSecret(t *testing.T) {
	cfg := validProductionConfig()
	cfg.JWT.Secret = strings.Repeat("x", 31)

	err := cfg.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "jwt.secret")
}

func TestValidate_ProductionRejectsDevSecret(t *testing.T) {
	cfg := validProductionConfig()
	cfg.JWT.Secret = DevJWTSecret

	require.Error(t, cfg.Validate())
}

func TestValidate_ProductionRejectsWildcardCORS(t *testing.T) {
	cfg := validProductionConfig()
	cfg.Server.CORSOrigins = []string{"*"}

	err := cfg.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "server.cors_origins")
}

func TestValidate_ProductionRejectsDocsEnabled(t *testing.T) {
	cfg := validProductionConfig()
	cfg.Docs.Enabled = true

	err := cfg.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "docs.enabled")
}

func TestValidate_ProductionAcceptsValidConfig(t *testing.T) {
	assert.NoError(t, validProductionConfig().Validate())
}

func TestValidate_DevelopmentAllowsDefaults(t *testing.T) {
	dir := writeConfig(t, baseConfig)

	_, err := Load(dir)

	assert.NoError(t, err)
}
