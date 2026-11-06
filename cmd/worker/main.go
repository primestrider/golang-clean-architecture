package main

import (
	"context"
	"golang-clean-architecture/internal/config"
	"golang-clean-architecture/internal/delivery/messaging"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/spf13/viper"
)

func main() {
	viperConfig := config.NewViper()
	logger := config.NewLogger(viperConfig)
	logger.Info("Starting worker service")

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Go(func() { RunUserConsumer(logger, viperConfig, ctx) })
	wg.Go(func() { RunContactConsumer(logger, viperConfig, ctx) })
	wg.Go(func() { RunAddressConsumer(logger, viperConfig, ctx) })

	terminateSignals := make(chan os.Signal, 1)
	signal.Notify(terminateSignals, syscall.SIGINT, syscall.SIGTERM)

	s := <-terminateSignals
	logger.Info("Got stop signal, shutting down worker gracefully", "signal", s.String())
	cancel()

	wg.Wait() // wait for all consumers to close their consumer groups
	logger.Info("Worker stopped")
}

func RunAddressConsumer(logger *slog.Logger, viperConfig *viper.Viper, ctx context.Context) {
	logger.Info("setup address consumer")
	addressConsumerGroup := config.NewKafkaConsumerGroup(viperConfig, logger)
	addressHandler := messaging.NewAddressConsumer(logger)
	messaging.ConsumeTopic(ctx, addressConsumerGroup, "addresses", logger, addressHandler.Consume)
}

func RunContactConsumer(logger *slog.Logger, viperConfig *viper.Viper, ctx context.Context) {
	logger.Info("setup contact consumer")
	contactConsumerGroup := config.NewKafkaConsumerGroup(viperConfig, logger)
	contactHandler := messaging.NewContactConsumer(logger)
	messaging.ConsumeTopic(ctx, contactConsumerGroup, "contacts", logger, contactHandler.Consume)
}

func RunUserConsumer(logger *slog.Logger, viperConfig *viper.Viper, ctx context.Context) {
	logger.Info("setup user consumer")
	userConsumerGroup := config.NewKafkaConsumerGroup(viperConfig, logger)
	userHandler := messaging.NewUserConsumer(logger)
	messaging.ConsumeTopic(ctx, userConsumerGroup, "users", logger, userHandler.Consume)
}
