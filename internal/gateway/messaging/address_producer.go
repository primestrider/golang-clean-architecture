package messaging

import (
	"github.com/primestrider/golang-clean-architecture/internal/model"

	"github.com/IBM/sarama"
	"log/slog"
)

type AddressProducer struct {
	Producer[*model.AddressEvent]
}

func NewAddressProducer(producer sarama.SyncProducer, log *slog.Logger) *AddressProducer {
	return &AddressProducer{
		Producer: Producer[*model.AddressEvent]{
			Producer: producer,
			Topic:    "addresses",
			Log:      log,
		},
	}
}
