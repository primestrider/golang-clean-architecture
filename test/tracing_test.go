//go:build integration

package test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/primestrider/golang-clean-architecture/internal/entity"
	"github.com/primestrider/golang-clean-architecture/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// TestTracedRequest_WritesTraceparentToOutbox: konteks trace dari request
// HTTP ikut tersimpan di header event outbox, sehingga trace bisa diteruskan
// sampai ke consumer.
func TestTracedRequest_WritesTraceparentToOutbox(t *testing.T) {
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(sdktrace.NewTracerProvider())
	t.Cleanup(func() { otel.SetTracerProvider(previous) })
	app, _, token := loggedIn(t)

	resp, raw := app.Do(t, http.MethodPost, "/api/v1/contacts", model.CreateContactRequest{FirstName: "Budi"}, token)
	require.Equal(t, http.StatusOK, resp.StatusCode, string(raw))

	var row entity.OutboxEvent
	require.NoError(t, app.DB.Where("event_type = ?", "contact.created").First(&row).Error)
	headers := map[string]string{}
	require.NoError(t, json.Unmarshal(row.Headers, &headers))
	assert.Regexp(t, `^00-[0-9a-f]{32}-[0-9a-f]{16}-0[01]$`, headers["traceparent"])
}
