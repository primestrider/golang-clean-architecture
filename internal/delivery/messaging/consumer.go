package messaging

import (
	"context"
	"log/slog"

	"github.com/IBM/sarama"
	"github.com/primestrider/golang-clean-architecture/internal/model"
)

// NewLoggingHandler membuat handler yang hanya mencatat event ke log.
// Didaftarkan untuk semua event sebagai contoh dan alat bantu debug.
func NewLoggingHandler(log *slog.Logger) Handler {
	return func(ctx context.Context, e model.Event) error {
		log.InfoContext(ctx, "event received",
			"event_id", e.EventID, "event_type", e.EventType, "aggregate_id", e.AggregateID)
		return nil
	}
}

// consumerGroupHandler menghubungkan Processor ke antarmuka
// sarama.ConsumerGroupHandler.
type consumerGroupHandler struct {
	processor *Processor
	log       *slog.Logger
}

// Setup dipanggil sarama saat sesi consumer group dimulai.
func (h *consumerGroupHandler) Setup(sarama.ConsumerGroupSession) error { return nil }

// Cleanup dipanggil sarama saat sesi consumer group berakhir.
func (h *consumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error { return nil }

// ConsumeClaim membaca pesan dari satu partisi secara berurutan. Offset
// ditandai hanya setelah pesan selesai ditangani (berhasil atau masuk DLQ).
// Bila pesan bahkan gagal dipindah ke DLQ, sesi dihentikan supaya pesan itu
// dibaca ulang alih-alih hilang.
func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case msg, ok := <-claim.Messages():
			if !ok {
				return nil
			}
			if err := h.processor.Process(session.Context(), msg); err != nil {
				h.log.Error("message not handled, will be redelivered", "topic", msg.Topic, "offset", msg.Offset, "error", err)
				return err
			}
			session.MarkMessage(msg, "")
		case <-session.Context().Done():
			return nil
		}
	}
}

// ConsumeTopics mengonsumsi topik-topik sampai ctx dibatalkan, lalu menutup
// consumer group. Fungsi ini memblokir, jadi jalankan di goroutine sendiri.
func ConsumeTopics(ctx context.Context, group sarama.ConsumerGroup, topics []string, processor *Processor, log *slog.Logger) {
	h := &consumerGroupHandler{processor: processor, log: log}

	go func() {
		for err := range group.Errors() {
			log.Error("consumer group error", "error", err)
		}
	}()

	// Consume perlu dipanggil ulang dalam loop: ia kembali setiap kali
	// terjadi rebalance consumer group.
	for ctx.Err() == nil {
		if err := group.Consume(ctx, topics, h); err != nil {
			log.Error("consume failed", "topics", topics, "error", err)
		}
	}

	log.Info("closing consumer group", "topics", topics)
	if err := group.Close(); err != nil {
		log.Error("close consumer group failed", "error", err)
	}
}
