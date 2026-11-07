package config

import (
	"context"
	"fmt"

	"github.com/primestrider/golang-clean-architecture/internal/observability"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// NewTracerProvider menyiapkan tracing OpenTelemetry dan mengembalikan
// fungsi shutdown yang wajib dipanggil saat aplikasi berhenti (untuk
// mengirim span yang masih tertahan di buffer).
//
// Propagasi W3C selalu dipasang. Bila otel.enabled false, tidak ada span
// yang direkam atau dikirim (tracer bawaan OTel adalah no-op), sehingga
// biayanya hampir nol.
func NewTracerProvider(ctx context.Context, cfg *Config) (func(context.Context) error, error) {
	observability.SetupPropagation()
	if !cfg.OTel.Enabled {
		return func(context.Context) error { return nil }, nil
	}

	opts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(cfg.OTel.Endpoint)}
	if cfg.OTel.Insecure {
		// Plaintext hanya bila diminta eksplisit; selain itu exporter memakai
		// TLS dengan sertifikat sistem.
		opts = append(opts, otlptracegrpc.WithInsecure())
	}
	exporter, err := otlptracegrpc.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("create otlp exporter: %w", err)
	}

	res := resource.NewWithAttributes(semconv.SchemaURL,
		semconv.ServiceName(cfg.OTel.ServiceName),
		semconv.DeploymentEnvironment(cfg.App.Env),
	)
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		// ParentBased: ikuti keputusan sampling dari layanan pemanggil bila
		// ada; jika tidak, ambil sampel sebesar sample_ratio (0.0–1.0).
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.OTel.SampleRatio))),
	)
	otel.SetTracerProvider(tp)
	return tp.Shutdown, nil
}
