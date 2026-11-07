package http

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/primestrider/golang-clean-architecture/internal/apperror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func doProblem(t *testing.T, handlerErr error) (int, string, map[string]any, string) {
	t.Helper()
	app := fiber.New(fiber.Config{ErrorHandler: NewErrorHandler(slog.New(slog.DiscardHandler))})
	app.Get("/x", func(c fiber.Ctx) error { return handlerErr })

	resp, err := app.Test(httptest.NewRequest("GET", "/x", nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var body map[string]any
	require.NoError(t, json.Unmarshal(raw, &body))
	return resp.StatusCode, resp.Header.Get("Content-Type"), body, string(raw)
}

func TestErrorHandler_ProblemDetails(t *testing.T) {
	status, ctype, body, _ := doProblem(t, apperror.NotFound("contact"))

	assert.Equal(t, 404, status)
	assert.Equal(t, "application/problem+json", ctype)
	assert.EqualValues(t, 404, body["status"])
	assert.Equal(t, "Not Found", body["title"])
	assert.Equal(t, "contact not found", body["detail"])
	assert.Equal(t, "/x", body["instance"])
	assert.Equal(t, "https://problems.example.com/not-found", body["type"])
	assert.Contains(t, body, "request_id")
}

func TestErrorHandler_InternalHidesCause(t *testing.T) {
	status, _, _, raw := doProblem(t, apperror.Internal(errors.New("dsn=root:pw")))

	assert.Equal(t, 500, status)
	assert.NotContains(t, raw, "dsn")
}

func TestErrorHandler_UnknownErrorIs500(t *testing.T) {
	status, _, body, raw := doProblem(t, errors.New("sql: secret table"))

	assert.Equal(t, 500, status)
	assert.Equal(t, "internal server error", body["detail"])
	assert.NotContains(t, raw, "secret")
}

func TestErrorHandler_ValidationIncludesFields(t *testing.T) {
	status, _, body, _ := doProblem(t, apperror.Validation([]apperror.FieldError{{Field: "email", Message: "must be a valid email"}}))

	assert.Equal(t, 400, status)
	errs := body["errors"].([]any)
	assert.Equal(t, "email", errs[0].(map[string]any)["field"])
}

func TestErrorHandler_MapsFiberErrors(t *testing.T) {
	status, _, body, _ := doProblem(t, fiber.ErrRequestEntityTooLarge)

	assert.Equal(t, 413, status)
	assert.Equal(t, "https://problems.example.com/payload-too-large", body["type"])
}
