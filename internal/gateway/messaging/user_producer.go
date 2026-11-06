package messaging

import (
	"golang-clean-architecture/internal/model"
	"log/slog"

	"github.com/IBM/sarama"
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
