package main

import (
	"fmt"
	"github.com/primestrider/golang-clean-architecture/internal/config"
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
	app := config.NewFiber(viperConfig, log)

	config.BootstrapWeb(&config.WebDeps{Config: viperConfig, Log: log, DB: db, App: app})

	webPort := viperConfig.Server.Port
	err = app.Listen(fmt.Sprintf(":%d", webPort))
	if err != nil {
		log.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
