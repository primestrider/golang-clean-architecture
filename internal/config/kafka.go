package config

import (
	"fmt"
	"log/slog"

	"github.com/IBM/sarama"
)

// NewKafkaConsumerGroup membuat consumer group Kafka. offset_reset
// "earliest" berarti consumer group baru membaca pesan dari awal topik.
func NewKafkaConsumerGroup(cfg *Config) (sarama.ConsumerGroup, error) {
	sc := sarama.NewConfig()
	sc.Consumer.Return.Errors = true
	sc.Consumer.Offsets.Initial = sarama.OffsetNewest
	if cfg.Kafka.OffsetReset == "earliest" {
		sc.Consumer.Offsets.Initial = sarama.OffsetOldest
	}

	group, err := sarama.NewConsumerGroup(cfg.Kafka.Brokers, cfg.Kafka.GroupID, sc)
	if err != nil {
		return nil, fmt.Errorf("create kafka consumer group: %w", err)
	}
	return group, nil
}

// NewKafkaProducer membuat producer sinkron yang menunggu konfirmasi semua
// replica (WaitForAll), sehingga pesan yang dilaporkan terkirim tidak hilang
// walau satu broker mati. Mengembalikan nil bila Kafka dimatikan.
func NewKafkaProducer(cfg *Config, log *slog.Logger) (sarama.SyncProducer, error) {
	if !cfg.Kafka.Enabled {
		log.Info("kafka producer is disabled")
		return nil, nil
	}

	sc := sarama.NewConfig()
	sc.Producer.Return.Successes = true
	sc.Producer.RequiredAcks = sarama.WaitForAll
	sc.Producer.Retry.Max = 3

	producer, err := sarama.NewSyncProducer(cfg.Kafka.Brokers, sc)
	if err != nil {
		return nil, fmt.Errorf("create kafka producer: %w", err)
	}
	return producer, nil
}
