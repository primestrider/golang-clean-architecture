package config

import (
	"github.com/go-playground/validator/v10"
)

func NewValidator(_ *Config) *validator.Validate {
	return validator.New()
}
