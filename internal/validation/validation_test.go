package validation

import (
	"testing"

	"github.com/primestrider/golang-clean-architecture/internal/apperror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sample struct {
	Email    string `json:"email" validate:"required,email"`
	Name     string `json:"name" validate:"max=3"`
	Password string `json:"password" validate:"min=8"`
	Hidden   string `json:"-" validate:"required"`
}

func TestValidator_FieldErrorsUseJSONNames(t *testing.T) {
	v := New()

	err := v.Struct(sample{Email: "x", Name: "abcd", Password: "short", Hidden: "ok"})

	appErr, ok := apperror.As(err)
	require.True(t, ok)
	assert.Equal(t, apperror.CodeValidation, appErr.Code)
	assert.Equal(t, []apperror.FieldError{
		{Field: "email", Message: "must be a valid email"},
		{Field: "name", Message: "must be at most 3 characters"},
		{Field: "password", Message: "must be at least 8 characters"},
	}, appErr.Fields)
}

func TestValidator_RequiredMessage(t *testing.T) {
	err := New().Struct(sample{Name: "a", Password: "longenough", Hidden: "ok"})

	appErr, _ := apperror.As(err)
	assert.Equal(t, []apperror.FieldError{{Field: "email", Message: "is required"}}, appErr.Fields)
}

func TestValidator_ValidStructReturnsNil(t *testing.T) {
	assert.NoError(t, New().Struct(sample{Email: "a@b.co", Name: "a", Password: "longenough", Hidden: "x"}))
}
