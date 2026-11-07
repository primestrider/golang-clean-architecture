package apperror

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInternal_HidesCauseFromMessage(t *testing.T) {
	cause := errors.New("dsn=root:pw")

	err := Internal(cause)

	assert.Equal(t, CodeInternal, err.Code)
	assert.Equal(t, "internal server error", err.Message)
	assert.ErrorIs(t, err, cause)
}

func TestFrom_WrapsUnknownErrorAsInternal(t *testing.T) {
	err := From(errors.New("boom"))

	assert.Equal(t, CodeInternal, err.Code)
}

func TestFrom_KeepsAppErrorThroughWrapping(t *testing.T) {
	original := NotFound("contact")

	err := From(fmt.Errorf("layer: %w", original))

	assert.Same(t, original, err)
	assert.Equal(t, "contact not found", err.Message)
}
