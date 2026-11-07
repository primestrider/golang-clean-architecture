package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/IBM/sarama"
	"github.com/primestrider/golang-clean-architecture/internal/model"
	"github.com/primestrider/golang-clean-architecture/internal/observability"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// EventHandler memproses satu event yang sudah di-decode. Diimplementasikan
// oleh Registry.
type EventHandler interface {
	Handle(ctx context.Context, e model.Event) error
}

// Processor memproses satu pesan Kafka: decode, panggil handler dengan
// retry, dan kirim ke dead-letter queue (DLQ) bila tetap gagal.
//
// DLQ mencegah satu pesan rusak atau satu dependency yang mati membuat
// seluruh partisi macet: pesan yang gagal terus dipindahkan ke topik
// "<topik>.dlq" untuk diperiksa manual, lalu consumer lanjut ke pesan
// berikutnya.
type Processor struct {
	handler        EventHandler
	dlq            sarama.SyncProducer
	maxRetries     int
	initialBackoff time.Duration
	log            *slog.Logger
	metrics        *observability.Metrics
}

// NewProcessor membuat Processor. maxRetries adalah jumlah percobaan ulang
// setelah percobaan pertama; jeda antar percobaan naik dua kali lipat mulai
// dari initialBackoff (exponential backoff).
func NewProcessor(handler EventHandler, dlq sarama.SyncProducer, maxRetries int, initialBackoff time.Duration, log *slog.Logger) *Processor {
	return &Processor{handler: handler, dlq: dlq, maxRetries: maxRetries, initialBackoff: initialBackoff, log: log}
}

// WithMetrics mengaktifkan pencatatan metrik consumer.
func (p *Processor) WithMetrics(m *observability.Metrics) *Processor {
	p.metrics = m
	return p
}

// record mencatat hasil pemrosesan pesan ke metrik (bila aktif).
func (p *Processor) record(topic, result string) {
	if p.metrics == nil {
		return
	}
	p.metrics.MessagesProcessed.WithLabelValues(topic, result).Inc()
	if result == "dlq" {
		p.metrics.MessagesDLQ.WithLabelValues(topic).Inc()
	}
}

// Process memproses satu pesan. Mengembalikan nil bila pesan selesai
// ditangani (berhasil, atau sudah dipindah ke DLQ) sehingga offset boleh
// di-commit. Mengembalikan error hanya bila pesan bahkan tidak bisa
// dipindah ke DLQ; offset tidak di-commit dan pesan akan dibaca ulang.
func (p *Processor) Process(ctx context.Context, msg *sarama.ConsumerMessage) error {
	ctx = observability.ExtractContext(ctx, headersOf(msg))
	ctx, span := otel.Tracer("messaging").Start(ctx, "consume "+msg.Topic, trace.WithSpanKind(trace.SpanKindConsumer))
	defer span.End()

	var e model.Event
	if err := json.Unmarshal(msg.Value, &e); err != nil {
		p.log.ErrorContext(ctx, "undecodable message, sending to DLQ", "topic", msg.Topic, "error", err)
		p.record(msg.Topic, "dlq")
		return p.toDLQ(msg, err, 0)
	}

	var err error
	attempts := 0
	backoff := p.initialBackoff
	for attempts <= p.maxRetries {
		attempts++
		if err = p.handler.Handle(ctx, e); err == nil {
			p.record(msg.Topic, "success")
			return nil
		}
		p.log.WarnContext(ctx, "handle event failed",
			"event_id", e.EventID, "event_type", e.EventType, "attempt", attempts, "error", err)
		if attempts > p.maxRetries {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		backoff *= 2
	}

	p.log.ErrorContext(ctx, "event failed after retries, sending to DLQ",
		"event_id", e.EventID, "event_type", e.EventType, "attempts", attempts)
	p.record(msg.Topic, "dlq")
	return p.toDLQ(msg, err, attempts)
}

// toDLQ mengirim pesan asli ke topik DLQ beserta alasan kegagalannya.
func (p *Processor) toDLQ(msg *sarama.ConsumerMessage, cause error, attempts int) error {
	headers := []sarama.RecordHeader{
		{Key: []byte("x-error"), Value: []byte(cause.Error())},
		{Key: []byte("x-original-topic"), Value: []byte(msg.Topic)},
		{Key: []byte("x-attempts"), Value: []byte(strconv.Itoa(attempts))},
	}
	for _, h := range msg.Headers {
		headers = append(headers, *h)
	}
	_, _, err := p.dlq.SendMessage(&sarama.ProducerMessage{
		Topic:   msg.Topic + ".dlq",
		Key:     sarama.ByteEncoder(msg.Key),
		Value:   sarama.ByteEncoder(msg.Value),
		Headers: headers,
	})
	if err != nil {
		return fmt.Errorf("send to dlq: %w", err)
	}
	return nil
}

// headersOf mengubah header pesan Kafka menjadi map.
func headersOf(msg *sarama.ConsumerMessage) map[string]string {
	out := make(map[string]string, len(msg.Headers))
	for _, h := range msg.Headers {
		out[string(h.Key)] = string(h.Value)
	}
	return out
}
