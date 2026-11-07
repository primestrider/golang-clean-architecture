package model

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Event adalah amplop (envelope) standar untuk semua event domain.
//
// Semua event berbagi bentuk yang sama sehingga consumer bisa membaca
// metadata (jenis event, entity, pelaku) tanpa tahu isi Data. EventID unik
// per event dan dipakai consumer untuk idempotency: event yang terkirim dua
// kali (at-least-once) cukup diproses sekali.
type Event struct {
	EventID    string    `json:"event_id"`
	EventType  string    `json:"event_type"`
	OccurredAt time.Time `json:"occurred_at"`
	// AggregateID adalah id entity yang berubah. Dipakai sebagai key pesan
	// Kafka, sehingga semua event milik entity yang sama masuk ke partisi
	// yang sama dan urutannya terjaga.
	AggregateID string `json:"aggregate_id"`
	// ActorID adalah user yang memicu event; kosong untuk aksi tanpa login.
	ActorID string          `json:"actor_id,omitempty"`
	Data    json.RawMessage `json:"data"`
}

// NewEvent membuat event baru dengan id dan waktu terjadi saat ini.
func NewEvent(eventType, aggregateID, actorID string, data any) (Event, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return Event{}, err
	}
	return Event{
		EventID:     uuid.NewString(),
		EventType:   eventType,
		OccurredAt:  time.Now().UTC().Truncate(time.Millisecond),
		AggregateID: aggregateID,
		ActorID:     actorID,
		Data:        raw,
	}, nil
}

// topics memetakan awalan jenis event ke topik Kafka.
var topics = map[string]string{
	"user":    "users",
	"contact": "contacts",
	"address": "addresses",
}

// TopicFor mengembalikan topik Kafka untuk jenis event, berdasarkan awalan
// sebelum titik ("contact.created" -> "contacts"). Jenis yang tidak dikenal
// masuk ke topik "events".
func TopicFor(eventType string) string {
	prefix, _, _ := strings.Cut(eventType, ".")
	if topic, ok := topics[prefix]; ok {
		return topic
	}
	return "events"
}
