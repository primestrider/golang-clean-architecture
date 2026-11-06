package messaging

import (
	"encoding/json"
	"golang-clean-architecture/internal/model"
	"log/slog"

	"github.com/IBM/sarama"
)

type ContactConsumer struct {
	Log *slog.Logger
}

func NewContactConsumer(log *slog.Logger) *ContactConsumer {
	return &ContactConsumer{
		Log: log,
	}
}

func (c ContactConsumer) Consume(message *sarama.ConsumerMessage) error {
	ContactEvent := new(model.ContactEvent)
	if err := json.Unmarshal(message.Value, ContactEvent); err != nil {
		c.Log.Error("error unmarshalling Contact event", "error", err)
		return err
	}

	// TODO process event
	c.Log.Info("Received event", "topic", "contacts", "event", ContactEvent, "partition", message.Partition)
	return nil
}
