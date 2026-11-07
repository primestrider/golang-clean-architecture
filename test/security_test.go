//go:build integration

package test

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/primestrider/golang-clean-architecture/internal/config"
	"github.com/primestrider/golang-clean-architecture/internal/model"
	"github.com/primestrider/golang-clean-architecture/test/testsupport"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rawRequest mengirim request dengan body dan header mentah, untuk kasus
// yang tidak bisa dibentuk lewat TestApp.Do (JSON rusak, Content-Type salah).
func rawRequest(t *testing.T, app *testsupport.TestApp, method, path, body string, headers map[string]string) (*http.Response, string) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := app.App.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp, string(raw)
}

func TestAuth_MissingBearer401(t *testing.T) {
	app := testsupport.NewWebApp(t)

	resp, body := rawRequest(t, app, http.MethodGet, "/api/v1/users/_current", "", nil)

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Contains(t, body, "missing bearer token")
}

func TestAuth_AlgNoneToken401(t *testing.T) {
	app := testsupport.NewWebApp(t)
	user := app.CreateUser(t, "eko@example.com", "rahasia123")
	claims := jwt.MapClaims{"sub": user.ID, "role": "admin", "iss": app.Cfg.JWT.Issuer, "aud": app.Cfg.JWT.Audience, "exp": time.Now().Add(time.Hour).Unix()}
	token, err := jwt.NewWithClaims(jwt.SigningMethodNone, claims).SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	resp, _ := app.Do(t, http.MethodGet, "/api/v1/users/_current", nil, token)

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestLoginRateLimit_SixthAttempt429WithRetryAfter(t *testing.T) {
	app := testsupport.NewWebApp(t)
	app.CreateUser(t, "eko@example.com", "rahasia123")
	body := model.LoginRequest{Email: "eko@example.com", Password: "salahsalah"}

	for range 5 {
		resp, _ := app.Do(t, http.MethodPost, "/api/v1/auth/login", body, "")
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	}
	resp, raw := app.Do(t, http.MethodPost, "/api/v1/auth/login", body, "")

	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode, string(raw))
	assert.NotEmpty(t, resp.Header.Get("Retry-After"))
	assert.Equal(t, "application/problem+json", resp.Header.Get("Content-Type"))
}

// TestRateLimit_IgnoresSpoofedXForwardedFor: tanpa trusted proxy, header
// X-Forwarded-For buatan client tidak boleh dipakai sebagai IP, sehingga
// tidak bisa dipakai untuk mengakali rate limit.
func TestRateLimit_IgnoresSpoofedXForwardedFor(t *testing.T) {
	app := testsupport.NewWebApp(t)

	// Email berbeda di setiap request, sehingga hanya limiter per IP yang
	// bisa menghentikannya. Bila X-Forwarded-For palsu dipercaya, setiap
	// request akan terlihat datang dari IP berbeda dan tidak pernah dibatasi.
	var last *http.Response
	for i := range app.Cfg.RateLimit.AuthIPPerMinute + 1 {
		body := `{"email":"user` + strconv.Itoa(i) + `@example.com","password":"salahsalah"}`
		last, _ = rawRequest(t, app, http.MethodPost, "/api/v1/auth/login", body, map[string]string{
			"Content-Type":    "application/json",
			"X-Forwarded-For": "10.0.0." + strconv.Itoa(i),
		})
	}

	assert.Equal(t, http.StatusTooManyRequests, last.StatusCode)
}

func TestMalformedJSON400(t *testing.T) {
	app := testsupport.NewWebApp(t)

	resp, body := rawRequest(t, app, http.MethodPost, "/api/v1/users", `{"email":`, map[string]string{"Content-Type": "application/json"})

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, body)
	assert.Equal(t, "application/problem+json", resp.Header.Get("Content-Type"))
}

// TestOversizedBody413 memakai listener TCP sungguhan dan hanya mengirim
// header Content-Length yang melebihi batas. Server menolak berdasarkan
// header itu tanpa menunggu body, sehingga test tidak bergantung pada
// waktu penutupan koneksi.
func TestOversizedBody413(t *testing.T) {
	app := testsupport.NewWebApp(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() { _ = app.App.Listener(ln, fiber.ListenConfig{DisableStartupMessage: true}) }()
	t.Cleanup(func() { _ = app.App.Shutdown() })

	conn, err := net.Dial("tcp", ln.Addr().String())
	require.NoError(t, err)
	defer conn.Close()
	request := "POST /api/v1/users HTTP/1.1\r\n" +
		"Host: test\r\n" +
		"Content-Type: application/json\r\n" +
		"Content-Length: 2097152\r\n\r\n"
	_, err = conn.Write([]byte(request))
	require.NoError(t, err)

	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode)
	assert.Equal(t, "application/problem+json", resp.Header.Get("Content-Type"))
}

func TestWrongContentType415(t *testing.T) {
	app := testsupport.NewWebApp(t)

	resp, body := rawRequest(t, app, http.MethodPost, "/api/v1/users", "email=a@b.co", map[string]string{"Content-Type": "text/plain"})

	assert.Equal(t, http.StatusUnsupportedMediaType, resp.StatusCode, body)
	assert.Equal(t, "application/problem+json", resp.Header.Get("Content-Type"))
}

func TestPanicRecovered500(t *testing.T) {
	app := testsupport.NewWebApp(t)
	app.App.Get("/test-panic", func(fiber.Ctx) error { panic("boom") })

	resp, body := rawRequest(t, app, http.MethodGet, "/test-panic", "", nil)

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.NotContains(t, body, "boom")

	resp, _ = rawRequest(t, app, http.MethodGet, "/api/v1/users/_current", "", nil)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "app must keep serving after a panic")
}

func TestSecurityHeadersPresent(t *testing.T) {
	app := testsupport.NewWebApp(t)

	resp, _ := rawRequest(t, app, http.MethodGet, "/api/v1/users/_current", "", nil)

	assert.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))
	assert.NotEmpty(t, resp.Header.Get("X-Frame-Options"))
}

func TestRequestIDEchoed(t *testing.T) {
	app := testsupport.NewWebApp(t)

	resp, body := rawRequest(t, app, http.MethodGet, "/api/v1/users/_current", "", map[string]string{"X-Request-ID": "abc-123"})

	assert.Equal(t, "abc-123", resp.Header.Get("X-Request-ID"))
	assert.Contains(t, body, `"request_id":"abc-123"`)
}

func TestRequestIDInvalidIsReplaced(t *testing.T) {
	app := testsupport.NewWebApp(t)

	resp, _ := rawRequest(t, app, http.MethodGet, "/api/v1/users/_current", "", map[string]string{"X-Request-ID": "<script>"})

	assert.NotEqual(t, "<script>", resp.Header.Get("X-Request-ID"))
	assert.NotEmpty(t, resp.Header.Get("X-Request-ID"))
}

func TestCORS_AllowsConfiguredOriginOnly(t *testing.T) {
	app := testsupport.NewWebApp(t)

	allowed, _ := rawRequest(t, app, http.MethodOptions, "/api/v1/users", "", map[string]string{
		"Origin": "http://localhost:5173", "Access-Control-Request-Method": "POST",
	})
	denied, _ := rawRequest(t, app, http.MethodOptions, "/api/v1/users", "", map[string]string{
		"Origin": "https://evil.example.com", "Access-Control-Request-Method": "POST",
	})

	assert.Equal(t, "http://localhost:5173", allowed.Header.Get("Access-Control-Allow-Origin"))
	assert.Empty(t, denied.Header.Get("Access-Control-Allow-Origin"))
}

// TestLoginRateLimit_PerIPAcrossDifferentEmails: satu IP yang mencoba banyak
// email berbeda (credential stuffing) tetap dibatasi oleh limiter per IP.
func TestLoginRateLimit_PerIPAcrossDifferentEmails(t *testing.T) {
	app := testsupport.NewWebApp(t)
	limit := app.Cfg.RateLimit.AuthIPPerMinute

	var last *http.Response
	for i := range limit + 1 {
		last, _ = app.Do(t, http.MethodPost, "/api/v1/auth/login",
			model.LoginRequest{Email: "user" + strconv.Itoa(i) + "@example.com", Password: "salahsalah"}, "")
	}

	assert.Equal(t, http.StatusTooManyRequests, last.StatusCode)
}

// TestLoginRateLimit_DistributedFailuresPerEmail: banyak IP (lewat proxy
// tepercaya) yang menebak password satu akun tetap dibatasi oleh limiter
// kegagalan per email.
func TestLoginRateLimit_DistributedFailuresPerEmail(t *testing.T) {
	app := testsupport.NewWebApp(t, func(cfg *config.Config) {
		cfg.Server.TrustedProxies = []string{"0.0.0.0", "127.0.0.1"}
	})
	app.CreateUser(t, "eko@example.com", "rahasia123")
	body := `{"email":"eko@example.com","password":"salahsalah"}`

	var last *http.Response
	for i := range app.Cfg.RateLimit.AuthEmailFailuresPerMinute + 1 {
		last, _ = rawRequest(t, app, http.MethodPost, "/api/v1/auth/login", body, map[string]string{
			"Content-Type":    "application/json",
			"X-Forwarded-For": "203.0.113." + strconv.Itoa(i+1),
		})
	}

	assert.Equal(t, http.StatusTooManyRequests, last.StatusCode)
}

// TestMetricsNotOnPublicPort: metrik hanya tersedia di port admin, bukan di
// API publik.
func TestMetricsNotOnPublicPort(t *testing.T) {
	app := testsupport.NewWebApp(t)

	resp, _ := rawRequest(t, app, http.MethodGet, "/metrics", "", nil)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// TestHTTPMetricsRecordedForAPIRequests: request ke API tercatat di metrik
// dengan label template route.
func TestHTTPMetricsRecordedForAPIRequests(t *testing.T) {
	app := testsupport.NewWebApp(t)
	user := app.CreateUser(t, "eko@example.com", "rahasia123")

	resp, _ := app.Do(t, http.MethodGet, "/api/v1/users/_current", nil, app.TokenFor(t, user))
	require.Equal(t, http.StatusOK, resp.StatusCode)

	assert.InDelta(t, 1, testutil.ToFloat64(app.Metrics.HTTPRequests.WithLabelValues("GET", "/api/v1/users/_current", "200")), 0)
}

// TestRefreshRateLimit_AllowsManyUsersBehindOneIP: refresh dipanggil semua
// pengguna setiap ~15 menit. Banyak pengguna di belakang satu IP (kantor,
// NAT operator seluler) tidak boleh langsung kena 429 setelah 5 refresh.
func TestRefreshRateLimit_AllowsManyUsersBehindOneIP(t *testing.T) {
	app := testsupport.NewWebApp(t)

	for i := range 10 {
		resp, _ := app.Do(t, http.MethodPost, "/api/v1/auth/refresh", model.RefreshRequest{RefreshToken: "token-" + strconv.Itoa(i)}, "")
		require.NotEqual(t, http.StatusTooManyRequests, resp.StatusCode, "refresh #%d was rate limited", i+1)
	}
}
