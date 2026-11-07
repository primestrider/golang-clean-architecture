package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
)

func logLine(t *testing.T, ctx context.Context) map[string]any {
	t.Helper()
	var buf bytes.Buffer
	logger := slog.New(NewContextHandler(slog.NewJSONHandler(&buf, nil)))

	logger.InfoContext(ctx, "hello")

	var out map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &out))
	return out
}

func TestContextHandler_AddsRequestAndUserID(t *testing.T) {
	ctx := WithUserID(WithRequestID(context.Background(), "r1"), "u1")

	out := logLine(t, ctx)

	assert.Equal(t, "r1", out["request_id"])
	assert.Equal(t, "u1", out["user_id"])
}

func TestContextHandler_AddsTraceAndSpanID(t *testing.T) {
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{0x01},
		SpanID:     trace.SpanID{0x02},
		TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)

	out := logLine(t, ctx)

	assert.Equal(t, sc.TraceID().String(), out["trace_id"])
	assert.Equal(t, sc.SpanID().String(), out["span_id"])
}

func TestContextHandler_OmitsMissingValues(t *testing.T) {
	out := logLine(t, context.Background())

	assert.NotContains(t, out, "request_id")
	assert.NotContains(t, out, "trace_id")
}

func TestContextHandler_WithAttrsKeepsContextEnrichment(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewContextHandler(slog.NewJSONHandler(&buf, nil))).With("component", "x")

	logger.InfoContext(WithRequestID(context.Background(), "r2"), "hello")

	assert.Contains(t, buf.String(), `"request_id":"r2"`)
	assert.Contains(t, buf.String(), `"component":"x"`)
}
