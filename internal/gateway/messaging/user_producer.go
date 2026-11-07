package messaging

import (
	"github.com/primestrider/golang-clean-architecture/internal/model"

	"github.com/IBM/sarama"
	"log/slog"
)

type UserProducer struct {
	Producer[*model.UserEvent]
}

func NewUserProducer(producer sarama.SyncProducer, log *slog.Logger) *UserProducer {
	return &UserProducer{
		Producer: Producer[*model.UserEvent]{
			Producer: producer,
			Topic:    "users",
			Log:      log,
		},
	}
}
