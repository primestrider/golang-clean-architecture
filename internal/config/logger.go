package config

import (
	"log/slog"
	"os"
	"strings"

	"github.com/primestrider/golang-clean-architecture/internal/observability"
)

// NewLogger membuat logger slog sesuai konfigurasi.
//
// Format "json" cocok untuk production karena mudah diolah sistem log
// (Loki, ELK, CloudWatch). Format "text" lebih enak dibaca saat development.
// Handler-nya dibungkus observability.ContextHandler supaya request id,
// user id, dan trace id dari context otomatis ikut di setiap baris log.
func NewLogger(cfg *Config) *slog.Logger {
	opts := &slog.HandlerOptions{Level: parseLevel(cfg.Log.Level)}

	var handler slog.Handler
	if strings.EqualFold(cfg.Log.Format, "json") {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	logger := slog.New(observability.NewContextHandler(handler)).With("service", cfg.App.Name)
	slog.SetDefault(logger)
	return logger
}

// parseLevel mengubah teks level (debug, info, warn, error) menjadi slog.Level.
// Nilai yang tidak dikenal jatuh ke info.
func parseLevel(level string) slog.Level {
	var l slog.Level
	if err := l.UnmarshalText([]byte(level)); err != nil {
		return slog.LevelInfo
	}
	return l
}
