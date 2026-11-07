package security

import (
	"crypto/rand"
	"crypto/rsa"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/primestrider/golang-clean-architecture/internal/apperror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	testSecret = []byte(strings.Repeat("s", 32))
	fixedNow   = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
)

func newIssuer(now time.Time) *TokenIssuer {
	return NewTokenIssuer(testSecret, "gca", "gca-api", 15*time.Minute, func() time.Time { return now })
}

func assertUnauthorized(t *testing.T, err error) {
	t.Helper()
	appErr, ok := apperror.As(err)
	require.True(t, ok, "expected apperror, got %v", err)
	assert.Equal(t, apperror.CodeUnauthorized, appErr.Code)
}

func TestIssue_RoundTrip(t *testing.T) {
	issuer := newIssuer(time.Now())

	token, expiresAt, err := issuer.Issue("user-1", "admin")
	require.NoError(t, err)
	claims, err := issuer.Parse(token)

	require.NoError(t, err)
	assert.Equal(t, "user-1", claims.UserID)
	assert.Equal(t, "admin", claims.Role)
	assert.NotEmpty(t, claims.ID, "jti must be set")
	assert.WithinDuration(t, time.Now().Add(15*time.Minute), expiresAt, 5*time.Second)
}

func TestParse_RejectsExpired(t *testing.T) {
	token, _, err := newIssuer(fixedNow.Add(-time.Hour)).Issue("user-1", "user")
	require.NoError(t, err)

	_, err = newIssuer(fixedNow).Parse(token)

	assertUnauthorized(t, err)
}

func TestParse_RejectsWrongAudience(t *testing.T) {
	other := NewTokenIssuer(testSecret, "gca", "another-api", 15*time.Minute, time.Now)
	token, _, err := other.Issue("user-1", "user")
	require.NoError(t, err)

	_, err = newIssuer(time.Now()).Parse(token)

	assertUnauthorized(t, err)
}

func TestParse_RejectsWrongSecret(t *testing.T) {
	other := NewTokenIssuer([]byte(strings.Repeat("x", 32)), "gca", "gca-api", 15*time.Minute, time.Now)
	token, _, err := other.Issue("user-1", "user")
	require.NoError(t, err)

	_, err = newIssuer(time.Now()).Parse(token)

	assertUnauthorized(t, err)
}

func TestParse_RejectsAlgNone(t *testing.T) {
	claims := jwt.MapClaims{"sub": "user-1", "role": "admin", "iss": "gca", "aud": "gca-api", "exp": time.Now().Add(time.Hour).Unix()}
	token, err := jwt.NewWithClaims(jwt.SigningMethodNone, claims).SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	_, err = newIssuer(time.Now()).Parse(token)

	assertUnauthorized(t, err)
}

func TestParse_RejectsRS256Header(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	claims := jwt.MapClaims{"sub": "user-1", "role": "admin", "iss": "gca", "aud": "gca-api", "exp": time.Now().Add(time.Hour).Unix()}
	token, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(key)
	require.NoError(t, err)

	_, err = newIssuer(time.Now()).Parse(token)

	assertUnauthorized(t, err)
}

func TestParse_RejectsGarbage(t *testing.T) {
	_, err := newIssuer(time.Now()).Parse("not-a-jwt")

	assertUnauthorized(t, err)
}

func TestNewRefreshToken_43CharsBase64URLAndHashIs64Hex(t *testing.T) {
	plain, hash, err := NewRefreshToken()

	require.NoError(t, err)
	assert.Regexp(t, regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`), plain)
	assert.Regexp(t, regexp.MustCompile(`^[0-9a-f]{64}$`), hash)
	assert.Equal(t, hash, HashRefreshToken(plain))

	other, _, err := NewRefreshToken()
	require.NoError(t, err)
	assert.NotEqual(t, plain, other)
}

func TestComparePassword(t *testing.T) {
	hash, err := HashPassword("rahasia123")
	require.NoError(t, err)

	assert.True(t, ComparePassword(hash, "rahasia123"))
	assert.False(t, ComparePassword(hash, "salah"))
	assert.NotContains(t, hash, "rahasia123")
}

func TestCompareDummy_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() { CompareDummy("anything") })
}
