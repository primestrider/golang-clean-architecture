//go:build integration

package test

import (
	"bytes"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/primestrider/golang-clean-architecture/internal/config"
	"github.com/primestrider/golang-clean-architecture/internal/model"
	"github.com/primestrider/golang-clean-architecture/test/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func appWithStorage(t *testing.T) *testsupport.TestApp {
	endpoint, ak, sk := testsupport.StartS3(t)
	return testsupport.NewWebApp(t, func(cfg *config.Config) {
		cfg.Storage.Enabled = true
		cfg.Storage.Endpoint = endpoint
		cfg.Storage.AccessKey = ak
		cfg.Storage.SecretKey = sk
		cfg.Storage.Bucket = "avatars"
	})
}

func uploadAvatar(t *testing.T, app *testsupport.TestApp, token, filename string, content []byte) (*http.Response, []byte) {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("file", filename)
	require.NoError(t, err)
	_, _ = part.Write(content)
	require.NoError(t, w.Close())

	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/_current/avatar", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.App.Test(req, fiber.TestConfig{Timeout: 30 * time.Second})
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp, raw
}

func TestAvatar_UploadAndFetchPresignedURL(t *testing.T) {
	app := appWithStorage(t)
	user := app.CreateUser(t, "eko@example.com", "rahasia123")
	token := app.TokenFor(t, user)
	var img bytes.Buffer
	require.NoError(t, png.Encode(&img, image.NewRGBA(image.Rect(0, 0, 4, 4))))

	resp, raw := uploadAvatar(t, app, token, "me.png", img.Bytes())
	require.Equal(t, http.StatusOK, resp.StatusCode, string(raw))
	avatarURL := testsupport.DecodeData[model.UserResponse](t, raw).AvatarURL
	require.NotEmpty(t, avatarURL)

	got, err := http.Get(avatarURL)
	require.NoError(t, err)
	defer got.Body.Close()
	stored, _ := io.ReadAll(got.Body)
	assert.Equal(t, http.StatusOK, got.StatusCode)
	assert.Equal(t, img.Bytes(), stored)

	_, raw = app.Do(t, http.MethodGet, "/api/v1/users/_current", nil, token)
	assert.NotEmpty(t, testsupport.DecodeData[model.UserResponse](t, raw).AvatarURL, "profile exposes a presigned avatar URL")
}

func TestAvatar_RejectsTextRenamedAsPNG(t *testing.T) {
	app := appWithStorage(t)
	user := app.CreateUser(t, "eko@example.com", "rahasia123")

	resp, _ := uploadAvatar(t, app, app.TokenFor(t, user), "evil.png", []byte("<html><script>alert(1)</script></html>"))

	assert.Equal(t, http.StatusUnsupportedMediaType, resp.StatusCode)
}

func TestAvatar_RouteAbsentWhenStorageDisabled(t *testing.T) {
	app := testsupport.NewWebApp(t)
	user := app.CreateUser(t, "eko@example.com", "rahasia123")

	resp, _ := uploadAvatar(t, app, app.TokenFor(t, user), "me.png", []byte("x"))

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
