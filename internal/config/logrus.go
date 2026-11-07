package config

import (
	"github.com/sirupsen/logrus"
)

func NewLogger(cfg *Config) *logrus.Logger {
	log := logrus.New()

	level, err := logrus.ParseLevel(cfg.Log.Level)
	if err != nil {
		level = logrus.InfoLevel
	}
	log.SetLevel(level)
	log.SetFormatter(&logrus.JSONFormatter{})

	return log
}
