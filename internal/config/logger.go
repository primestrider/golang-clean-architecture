package config

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/viper"
)

// NewLogger creates a JSON logger. log.level accepts debug, info, warn or error.
func NewLogger(viper *viper.Viper) *slog.Logger {
	var level slog.Level
	if err := level.UnmarshalText([]byte(viper.GetString("log.level"))); err != nil {
		panic(fmt.Errorf("invalid log.level: %w", err))
	}

	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
}
