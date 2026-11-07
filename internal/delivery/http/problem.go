package http

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/primestrider/golang-clean-architecture/internal/apperror"
	"github.com/primestrider/golang-clean-architecture/internal/observability"
)

// problemTypeBase adalah awalan URI untuk field "type" Problem Details.
// Ganti dengan alamat dokumentasi error milik Anda sendiri.
const problemTypeBase = "https://problems.example.com/"

// Problem adalah body error sesuai RFC 9457 (Problem Details for HTTP APIs).
// Dengan format standar ini, client cukup menulis satu penanganan error
// untuk semua endpoint.
type Problem struct {
	Type      string                `json:"type"`
	Title     string                `json:"title"`
	Status    int                   `json:"status"`
	Detail    string                `json:"detail,omitempty"`
	Instance  string                `json:"instance,omitempty"`
	RequestID string                `json:"request_id"`
	Errors    []apperror.FieldError `json:"errors,omitempty"`
}

// HTTPStatus memetakan apperror.Code ke status HTTP.
func HTTPStatus(code apperror.Code) int {
	switch code {
	case apperror.CodeBadRequest, apperror.CodeValidation:
		return http.StatusBadRequest
	case apperror.CodeUnauthorized:
		return http.StatusUnauthorized
	case apperror.CodeForbidden:
		return http.StatusForbidden
	case apperror.CodeNotFound:
		return http.StatusNotFound
	case apperror.CodeConflict:
		return http.StatusConflict
	case apperror.CodePayloadTooLarge:
		return http.StatusRequestEntityTooLarge
	case apperror.CodeUnsupportedMedia:
		return http.StatusUnsupportedMediaType
	case apperror.CodeRateLimited:
		return http.StatusTooManyRequests
	case apperror.CodeUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// fiberCode memetakan status error bawaan Fiber (body terlalu besar, route
// tidak ada, dan sebagainya) ke apperror.Code supaya bentuk responsnya
// tetap seragam.
func fiberCode(status int) apperror.Code {
	switch status {
	case http.StatusBadRequest:
		return apperror.CodeBadRequest
	case http.StatusUnauthorized:
		return apperror.CodeUnauthorized
	case http.StatusForbidden:
		return apperror.CodeForbidden
	case http.StatusNotFound, http.StatusMethodNotAllowed:
		return apperror.CodeNotFound
	case http.StatusRequestEntityTooLarge:
		return apperror.CodePayloadTooLarge
	case http.StatusUnsupportedMediaType:
		return apperror.CodeUnsupportedMedia
	case http.StatusTooManyRequests:
		return apperror.CodeRateLimited
	case http.StatusServiceUnavailable, http.StatusRequestTimeout:
		return apperror.CodeUnavailable
	default:
		return apperror.CodeInternal
	}
}

// toAppError mengubah error apa pun menjadi *apperror.Error.
func toAppError(err error) *apperror.Error {
	if fe, ok := errors.AsType[*fiber.Error](err); ok {
		code := fiberCode(fe.Code)
		if code == apperror.CodeInternal {
			return apperror.Internal(err)
		}
		return apperror.New(code, strings.ToLower(fe.Message))
	}
	return apperror.From(err)
}

// NewErrorHandler membuat error handler global Fiber yang menulis semua
// error sebagai application/problem+json.
//
// Error 5xx dicatat di level error beserta penyebab aslinya, tetapi client
// hanya menerima pesan generik. Error 4xx dicatat di level debug karena
// biasanya merupakan kesalahan client, bukan masalah server.
func NewErrorHandler(log *slog.Logger) fiber.ErrorHandler {
	return func(c fiber.Ctx, err error) error {
		appErr := toAppError(err)
		status := HTTPStatus(appErr.Code)
		ctx := c.Context()

		if status >= http.StatusInternalServerError {
			log.ErrorContext(ctx, "request failed", "error", err, "path", c.Path())
		} else {
			log.DebugContext(ctx, "request rejected", "error", err, "path", c.Path())
		}

		problem := Problem{
			Type:      problemTypeBase + strings.ReplaceAll(strings.ToLower(string(appErr.Code)), "_", "-"),
			Title:     http.StatusText(status),
			Status:    status,
			Detail:    appErr.Message,
			Instance:  c.Path(),
			RequestID: observability.RequestID(ctx),
			Errors:    appErr.Fields,
		}
		return c.Status(status).JSON(problem, "application/problem+json")
	}
}
