package messaging

import (
	"golang-clean-architecture/internal/model"
	"log/slog"

	"github.com/IBM/sarama"
)

type ContactProducer struct {
	Producer[*model.ContactEvent]
}

func NewContactProducer(producer sarama.SyncProducer, log *slog.Logger) *ContactProducer {
	return &ContactProducer{
		Producer: Producer[*model.ContactEvent]{
			Producer: producer,
			Topic:    "contacts",
			Log:      log,
		},
	}
}
