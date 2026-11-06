package main

import (
	"context"
	"fmt"
	"golang-clean-architecture/internal/config"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
)

func main() {
	viperConfig := config.NewViper()
	log := config.NewLogger(viperConfig)
	db := config.NewDatabase(viperConfig, log)
	validate := config.NewValidator(viperConfig)
	app := config.NewFiber(viperConfig, log)
	producer := config.NewKafkaProducer(viperConfig, log)

	config.Bootstrap(&config.BootstrapConfig{
		DB:       db,
		App:      app,
		Log:      log,
		Validate: validate,
		Config:   viperConfig,
		Producer: producer,
	})

	// cancelled on SIGINT/SIGTERM; Fiber then stops accepting requests and drains in-flight ones
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	webPort := viperConfig.GetInt("web.port")
	err := app.Listen(fmt.Sprintf(":%d", webPort), fiber.ListenConfig{
		EnablePrefork:   viperConfig.GetBool("web.prefork"),
		GracefulContext: ctx,
		ShutdownTimeout: 10 * time.Second,
	})
	if err != nil {
		log.Error("Failed to start server", "error", err)
		os.Exit(1)
	}

	// Listen returns after shutdown; release resources once in-flight requests are done
	if producer != nil {
		if err := producer.Close(); err != nil {
			log.Error("Failed to close kafka producer", "error", err)
		}
	}
	if sqlDB, err := db.DB(); err == nil {
		if err := sqlDB.Close(); err != nil {
			log.Error("Failed to close database", "error", err)
		}
	}
	log.Info("Server stopped")
}
