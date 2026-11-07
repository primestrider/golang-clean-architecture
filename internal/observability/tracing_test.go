package observability

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/trace"
)

func TestInjectExtract_RoundTrip(t *testing.T) {
	SetupPropagation()
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{0xaa, 0x01},
		SpanID:     trace.SpanID{0xbb, 0x02},
		TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)

	headers := InjectHeaders(ctx)
	restored := trace.SpanContextFromContext(ExtractContext(context.Background(), headers))

	assert.Contains(t, headers, "traceparent")
	assert.Equal(t, sc.TraceID(), restored.TraceID())
	assert.True(t, restored.IsRemote())
}

func TestInjectHeaders_EmptyWithoutSpan(t *testing.T) {
	SetupPropagation()

	assert.Empty(t, InjectHeaders(context.Background()))
}
