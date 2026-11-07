package messaging

import (
	"github.com/primestrider/golang-clean-architecture/internal/model"

	"github.com/IBM/sarama"
	"log/slog"
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
