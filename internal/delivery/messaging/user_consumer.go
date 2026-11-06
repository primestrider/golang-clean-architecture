package messaging

import (
	"encoding/json"
	"golang-clean-architecture/internal/model"
	"log/slog"

	"github.com/IBM/sarama"
)

type UserConsumer struct {
	Log *slog.Logger
}

func NewUserConsumer(log *slog.Logger) *UserConsumer {
	return &UserConsumer{
		Log: log,
	}
}

func (c UserConsumer) Consume(message *sarama.ConsumerMessage) error {
	UserEvent := new(model.UserEvent)
	if err := json.Unmarshal(message.Value, UserEvent); err != nil {
		c.Log.Error("error unmarshalling User event", "error", err)
		return err
	}

	// TODO process event
	c.Log.Info("Received event", "topic", "users", "event", UserEvent, "partition", message.Partition)
	return nil
}
