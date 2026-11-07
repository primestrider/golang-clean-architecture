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
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
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

// TestTracing_RedactsSearchFiltersFromQuery: filter pencarian kontak (nama,
// email, telepon) adalah data pribadi dan tidak boleh tersimpan di span.
func TestTracing_RedactsSearchFiltersFromQuery(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)))
	t.Cleanup(func() { otel.SetTracerProvider(previous) })
	app, _, token := loggedIn(t)

	resp, _ := app.Do(t, http.MethodGet, "/api/v1/contacts?email=secret%40example.com&phone=0812&page=1", nil, token)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var all string
	for _, span := range recorder.Ended() {
		for _, attr := range span.Attributes() {
			all += string(attr.Key) + "=" + attr.Value.String() + " "
		}
	}
	assert.Contains(t, all, "url.query=")
	assert.NotContains(t, all, "secret")
	assert.NotContains(t, all, "0812")
}
