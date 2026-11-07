package main

import (
	"fmt"
	"github.com/primestrider/golang-clean-architecture/internal/config"
	"github.com/primestrider/golang-clean-architecture/internal/validation"
	"os"
)

func main() {
	viperConfig, err := config.Load()
	if err != nil {
		panic(err)
	}
	log := config.NewLogger(viperConfig)
	db, err := config.NewDatabase(viperConfig, log)
	if err != nil {
		log.Error("startup failed", "error", err)
		os.Exit(1)
	}
	validate := validation.New()
	app := config.NewFiber(viperConfig, log)
	producer, err := config.NewKafkaProducer(viperConfig, log)
	if err != nil {
		log.Error("startup failed", "error", err)
		os.Exit(1)
	}

	config.Bootstrap(&config.BootstrapConfig{
		DB:       db,
		App:      app,
		Log:      log,
		Validate: validate,
		Config:   viperConfig,
		Producer: producer,
	})

	webPort := viperConfig.Server.Port
	err = app.Listen(fmt.Sprintf(":%d", webPort))
	if err != nil {
		log.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
