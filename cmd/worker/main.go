package main

import (
	"context"
	"github.com/primestrider/golang-clean-architecture/internal/config"
	"github.com/primestrider/golang-clean-architecture/internal/delivery/messaging"
	"os"
	"os/signal"
	"syscall"
	"time"

	"log/slog"
)

func main() {
	viperConfig, err := config.Load()
	if err != nil {
		panic(err)
	}
	logger := config.NewLogger(viperConfig)
	logger.Info("Starting worker service")

	ctx, cancel := context.WithCancel(context.Background())

	go RunUserConsumer(logger, viperConfig, ctx)
	go RunContactConsumer(logger, viperConfig, ctx)
	go RunAddressConsumer(logger, viperConfig, ctx)

	terminateSignals := make(chan os.Signal, 1)
	signal.Notify(terminateSignals, syscall.SIGINT, syscall.SIGKILL, syscall.SIGTERM)

	stop := false
	for !stop {
		select {
		case s := <-terminateSignals:
			logger.Info("shutdown signal received", "signal", s.String())
			cancel()
			stop = true
		}
	}

	time.Sleep(5 * time.Second) // wait for all consumers to finish processing
}

func RunAddressConsumer(logger *slog.Logger, viperConfig *config.Config, ctx context.Context) {
	logger.Info("setup address consumer")
	addressConsumerGroup, err := config.NewKafkaConsumerGroup(viperConfig)
	if err != nil {
		logger.Error("create consumer group failed", "error", err)
		return
	}
	addressHandler := messaging.NewAddressConsumer(logger)
	messaging.ConsumeTopic(ctx, addressConsumerGroup, "addresses", logger, addressHandler.Consume)
}

func RunContactConsumer(logger *slog.Logger, viperConfig *config.Config, ctx context.Context) {
	logger.Info("setup contact consumer")
	contactConsumerGroup, err := config.NewKafkaConsumerGroup(viperConfig)
	if err != nil {
		logger.Error("create consumer group failed", "error", err)
		return
	}
	contactHandler := messaging.NewContactConsumer(logger)
	messaging.ConsumeTopic(ctx, contactConsumerGroup, "contacts", logger, contactHandler.Consume)
}

func RunUserConsumer(logger *slog.Logger, viperConfig *config.Config, ctx context.Context) {
	logger.Info("setup user consumer")
	userConsumerGroup, err := config.NewKafkaConsumerGroup(viperConfig)
	if err != nil {
		logger.Error("create consumer group failed", "error", err)
		return
	}
	userHandler := messaging.NewUserConsumer(logger)
	messaging.ConsumeTopic(ctx, userConsumerGroup, "users", logger, userHandler.Consume)
}
