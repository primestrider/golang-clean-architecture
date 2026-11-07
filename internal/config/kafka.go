package config

import (
	"github.com/IBM/sarama"
	"github.com/sirupsen/logrus"
)

func NewKafkaConsumerGroup(config *Config, log *logrus.Logger) sarama.ConsumerGroup {
	saramaConfig := sarama.NewConfig()
	saramaConfig.Consumer.Return.Errors = true

	offsetReset := config.Kafka.OffsetReset
	if offsetReset == "earliest" {
		saramaConfig.Consumer.Offsets.Initial = sarama.OffsetOldest
	} else {
		saramaConfig.Consumer.Offsets.Initial = sarama.OffsetNewest
	}

	brokers := config.Kafka.Brokers
	groupID := config.Kafka.GroupID

	consumerGroup, err := sarama.NewConsumerGroup(brokers, groupID, saramaConfig)
	if err != nil {
		log.Fatalf("Failed to create consumer group: %v", err)
	}
	return consumerGroup
}

func NewKafkaProducer(config *Config, log *logrus.Logger) sarama.SyncProducer {
	if !config.Kafka.Enabled {
		log.Info("Kafka producer is disabled")
		return nil
	}

	saramaConfig := sarama.NewConfig()
	saramaConfig.Producer.Return.Successes = true
	saramaConfig.Producer.RequiredAcks = sarama.WaitForAll
	saramaConfig.Producer.Retry.Max = 3

	brokers := config.Kafka.Brokers

	producer, err := sarama.NewSyncProducer(brokers, saramaConfig)
	if err != nil {
		log.Fatalf("Failed to create producer: %v", err)
	}
	return producer
}
