package model

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEvent_Fields(t *testing.T) {
	before := time.Now()

	e, err := NewEvent("contact.created", "c1", "u1", map[string]string{"first_name": "Budi"})

	require.NoError(t, err)
	assert.Len(t, e.EventID, 36)
	assert.Equal(t, "contact.created", e.EventType)
	assert.Equal(t, "c1", e.AggregateID)
	assert.Equal(t, "u1", e.ActorID)
	assert.False(t, e.OccurredAt.Before(before.Truncate(time.Millisecond)))
	assert.JSONEq(t, `{"first_name":"Budi"}`, string(e.Data))
}

func TestEvent_JSONUsesSnakeCase(t *testing.T) {
	e, err := NewEvent("user.registered", "u1", "", map[string]string{"id": "u1"})
	require.NoError(t, err)

	raw, err := json.Marshal(e)
	require.NoError(t, err)

	var fields map[string]any
	require.NoError(t, json.Unmarshal(raw, &fields))
	for _, key := range []string{"event_id", "event_type", "occurred_at", "aggregate_id", "data"} {
		assert.Contains(t, fields, key)
	}
}

func TestTopicFor(t *testing.T) {
	assert.Equal(t, "users", TopicFor("user.registered"))
	assert.Equal(t, "contacts", TopicFor("contact.deleted"))
	assert.Equal(t, "addresses", TopicFor("address.updated"))
	assert.Equal(t, "events", TopicFor("unknown"))
}
