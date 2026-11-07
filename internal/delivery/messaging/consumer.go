package messaging

import (
	"context"
	"log/slog"

	"github.com/IBM/sarama"
)

// ConsumerHandler memproses satu pesan Kafka. Mengembalikan error berarti
// pesan gagal diproses dan offset-nya tidak di-commit.
type ConsumerHandler func(message *sarama.ConsumerMessage) error

// NewLoggingHandler membuat handler yang hanya mencatat pesan ke log.
// Berguna sebagai contoh dan untuk memastikan alur consumer berjalan.
func NewLoggingHandler(log *slog.Logger) ConsumerHandler {
	return func(message *sarama.ConsumerMessage) error {
		log.Info("received event",
			"topic", message.Topic,
			"partition", message.Partition,
			"offset", message.Offset,
			"key", string(message.Key),
		)
		return nil
	}
}

// consumerGroupHandler menghubungkan ConsumerHandler ke antarmuka
// sarama.ConsumerGroupHandler.
type consumerGroupHandler struct {
	handler ConsumerHandler
	log     *slog.Logger
}

// Setup dipanggil sarama saat sesi consumer group dimulai.
func (h *consumerGroupHandler) Setup(sarama.ConsumerGroupSession) error { return nil }

// Cleanup dipanggil sarama saat sesi consumer group berakhir.
func (h *consumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error { return nil }

// ConsumeClaim membaca pesan dari satu partisi. Offset hanya ditandai
// (MarkMessage) bila pesan berhasil diproses.
func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case message, ok := <-claim.Messages():
			if !ok {
				return nil
			}
			if err := h.handler(message); err != nil {
				h.log.Error("failed to process message", "topic", message.Topic, "error", err)
				continue
			}
			session.MarkMessage(message, "")
		case <-session.Context().Done():
			return nil
		}
	}
}

// ConsumeTopic mengonsumsi satu topik sampai ctx dibatalkan, lalu menutup
// consumer group. Fungsi ini memblokir, jadi jalankan di goroutine sendiri.
func ConsumeTopic(ctx context.Context, group sarama.ConsumerGroup, topic string, log *slog.Logger, handler ConsumerHandler) {
	h := &consumerGroupHandler{handler: handler, log: log}

	go func() {
		for err := range group.Errors() {
			log.Error("consumer group error", "topic", topic, "error", err)
		}
	}()

	// Consume perlu dipanggil ulang dalam loop: ia kembali setiap kali
	// terjadi rebalance consumer group.
	for ctx.Err() == nil {
		if err := group.Consume(ctx, []string{topic}, h); err != nil {
			log.Error("consume failed", "topic", topic, "error", err)
		}
	}

	log.Info("closing consumer group", "topic", topic)
	if err := group.Close(); err != nil {
		log.Error("close consumer group failed", "topic", topic, "error", err)
	}
}
