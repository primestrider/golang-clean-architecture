package outbox

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/IBM/sarama"
	"github.com/primestrider/golang-clean-architecture/internal/entity"
)

// KafkaDispatcher mengirim event outbox ke Kafka.
type KafkaDispatcher struct {
	producer sarama.SyncProducer
}

// NewKafkaDispatcher membuat KafkaDispatcher dari producer sinkron.
func NewKafkaDispatcher(producer sarama.SyncProducer) *KafkaDispatcher {
	return &KafkaDispatcher{producer: producer}
}

// Dispatch mengirim satu event dan menunggu konfirmasi broker.
//
// Key pesan adalah id aggregate, sehingga semua event milik entity yang sama
// masuk ke partisi yang sama dan dibaca consumer sesuai urutan. Header pesan
// berisi jenis event, id event, dan konteks tracing.
func (d *KafkaDispatcher) Dispatch(_ context.Context, e entity.OutboxEvent) error {
	headers := []sarama.RecordHeader{
		{Key: []byte("event_type"), Value: []byte(e.EventType)},
		{Key: []byte("event_id"), Value: []byte(e.ID)},
	}
	for k, v := range decodeHeaders(e.Headers) {
		headers = append(headers, sarama.RecordHeader{Key: []byte(k), Value: []byte(v)})
	}
	_, _, err := d.producer.SendMessage(&sarama.ProducerMessage{
		Topic:   e.Topic,
		Key:     sarama.StringEncoder(e.MsgKey),
		Value:   sarama.ByteEncoder(e.Payload),
		Headers: headers,
	})
	if err != nil && isUnavailable(err) {
		return Unavailable(err)
	}
	return err
}

// isUnavailable melaporkan apakah error berasal dari broker yang tidak bisa
// dihubungi, bukan dari pesan itu sendiri (misalnya pesan terlalu besar).
func isUnavailable(err error) bool {
	return IsTransient(err) ||
		errors.Is(err, sarama.ErrOutOfBrokers) ||
		errors.Is(err, sarama.ErrNotConnected) ||
		errors.Is(err, sarama.ErrBrokerNotAvailable) ||
		errors.Is(err, sarama.ErrLeaderNotAvailable) ||
		errors.Is(err, sarama.ErrNotLeaderForPartition) ||
		errors.Is(err, sarama.ErrRequestTimedOut) ||
		errors.Is(err, sarama.ErrClosedClient)
}

// decodeHeaders membaca kolom headers (JSON) menjadi map. Nilai yang tidak
// valid diabaikan supaya satu header rusak tidak menahan pengiriman event.
func decodeHeaders(raw []byte) map[string]string {
	headers := map[string]string{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &headers)
	}
	return headers
}
