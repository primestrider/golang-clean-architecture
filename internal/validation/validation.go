// Package validation membungkus go-playground/validator supaya hasil
// validasi berupa apperror dengan pesan per field yang ramah client.
package validation

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/primestrider/golang-clean-architecture/internal/apperror"
)

// Validator memvalidasi struct berdasarkan tag `validate`.
type Validator struct {
	v *validator.Validate
}

// New membuat Validator. Nama field di pesan error memakai nama di tag json
// (misalnya "first_name"), karena itulah nama yang dikenal client.
func New() *Validator {
	v := validator.New(validator.WithRequiredStructEnabled())
	v.RegisterTagNameFunc(func(f reflect.StructField) string {
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "-" || name == "" {
			return f.Name
		}
		return name
	})
	return &Validator{v: v}
}

// Struct memvalidasi s. Mengembalikan nil bila valid, atau *apperror.Error
// dengan Code VALIDATION_FAILED berisi daftar field yang salah.
func (val *Validator) Struct(s any) error {
	err := val.v.Struct(s)
	if err == nil {
		return nil
	}
	verrs, ok := errors.AsType[validator.ValidationErrors](err)
	if !ok {
		return apperror.Internal(err)
	}
	fields := make([]apperror.FieldError, 0, len(verrs))
	for _, fe := range verrs {
		fields = append(fields, apperror.FieldError{Field: fe.Field(), Message: message(fe)})
	}
	return apperror.Validation(fields)
}

// message menerjemahkan tag validator menjadi pesan bahasa Inggris sederhana.
func message(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "is required"
	case "email":
		return "must be a valid email"
	case "uuid", "uuid4":
		return "must be a valid UUID"
	case "min":
		if fe.Kind() == reflect.String {
			return fmt.Sprintf("must be at least %s characters", fe.Param())
		}
		return "must be at least " + fe.Param()
	case "max":
		if fe.Kind() == reflect.String {
			return fmt.Sprintf("must be at most %s characters", fe.Param())
		}
		return "must be at most " + fe.Param()
	case "oneof":
		return "must be one of: " + fe.Param()
	default:
		return "is invalid"
	}
}
