package test

import (
	"github.com/primestrider/golang-clean-architecture/internal/config"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
	"log/slog"
)

var app *fiber.App

var db *gorm.DB

var viperConfig *config.Config

var log *slog.Logger

func init() {
	var err error
	viperConfig, err = config.Load()
	if err != nil {
		panic(err)
	}
	log = config.NewLogger(viperConfig)
	app = config.NewFiber(viperConfig, log)
	db, err = config.NewDatabase(viperConfig, log)
	if err != nil {
		panic(err)
	}

	config.BootstrapWeb(&config.WebDeps{Config: viperConfig, Log: log, DB: db, App: app})
}
