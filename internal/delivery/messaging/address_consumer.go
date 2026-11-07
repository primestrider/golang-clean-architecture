package messaging

import (
	"encoding/json"
	"github.com/primestrider/golang-clean-architecture/internal/model"

	"github.com/IBM/sarama"
	"log/slog"
)

type AddressConsumer struct {
	Log *slog.Logger
}

func NewAddressConsumer(log *slog.Logger) *AddressConsumer {
	return &AddressConsumer{
		Log: log,
	}
}

func (c AddressConsumer) Consume(message *sarama.ConsumerMessage) error {
	addressEvent := new(model.AddressEvent)
	if err := json.Unmarshal(message.Value, addressEvent); err != nil {
		c.Log.Error("error unmarshalling address event", "error", err)
		return err
	}

	// TODO process event
	c.Log.Info("received event", "topic", "addresses", "event", addressEvent, "partition", message.Partition)
	return nil
}
