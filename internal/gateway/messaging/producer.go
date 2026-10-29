package messaging

import (
	"encoding/json"
	"golang-clean-architecture/internal/model"
	"log/slog"

	"github.com/IBM/sarama"
)

type Producer[T model.Event] struct {
	Producer sarama.SyncProducer
	Topic    string
	Log      *slog.Logger
}

func (p *Producer[T]) GetTopic() *string {
	return &p.Topic
}

func (p *Producer[T]) Send(event T) error {
	value, err := json.Marshal(event)
	if err != nil {
		p.Log.Error("failed to marshal event", "error", err)
		return err
	}

	message := &sarama.ProducerMessage{
		Topic: p.Topic,
		Key:   sarama.StringEncoder(event.GetId()),
		Value: sarama.ByteEncoder(value),
	}

	partition, offset, err := p.Producer.SendMessage(message)
	if err != nil {
		p.Log.Error("failed to produce message", "error", err)
		return err
	}

	p.Log.Debug("Message sent", "topic", p.Topic, "partition", partition, "offset", offset)
	return nil
}
