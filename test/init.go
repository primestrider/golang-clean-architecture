package test

import (
	"github.com/primestrider/golang-clean-architecture/internal/config"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

var app *fiber.App

var db *gorm.DB

var viperConfig *config.Config

var log *logrus.Logger

var validate *validator.Validate

func init() {
	var err error
	viperConfig, err = config.Load()
	if err != nil {
		panic(err)
	}
	log = config.NewLogger(viperConfig)
	validate = config.NewValidator(viperConfig)
	app = config.NewFiber(viperConfig)
	db = config.NewDatabase(viperConfig, log)
	producer := config.NewKafkaProducer(viperConfig, log)

	config.Bootstrap(&config.BootstrapConfig{
		DB:       db,
		App:      app,
		Log:      log,
		Validate: validate,
		Config:   viperConfig,
		Producer: producer,
	})
}
