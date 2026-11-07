package test

import (
	"github.com/primestrider/golang-clean-architecture/internal/config"

	"github.com/gofiber/fiber/v3"
	"github.com/primestrider/golang-clean-architecture/internal/validation"
	"gorm.io/gorm"
	"log/slog"
)

var app *fiber.App

var db *gorm.DB

var viperConfig *config.Config

var log *slog.Logger

var validate *validation.Validator

func init() {
	var err error
	viperConfig, err = config.Load()
	if err != nil {
		panic(err)
	}
	log = config.NewLogger(viperConfig)
	validate = validation.New()
	app = config.NewFiber(viperConfig, log)
	db, err = config.NewDatabase(viperConfig, log)
	if err != nil {
		panic(err)
	}
	producer, err := config.NewKafkaProducer(viperConfig, log)
	if err != nil {
		panic(err)
	}

	config.Bootstrap(&config.BootstrapConfig{
		DB:       db,
		App:      app,
		Log:      log,
		Validate: validate,
		Config:   viperConfig,
		Producer: producer,
	})
}
