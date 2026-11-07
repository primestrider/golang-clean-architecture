package observability

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// InjectHeaders menulis konteks tracing dari ctx (misalnya header
// "traceparent") ke map. Map ini disimpan bersama event di outbox, lalu
// dikirim sebagai header pesan, sehingga trace bisa diteruskan dari request
// HTTP ke consumer di worker.
//
// Bila tracing tidak aktif, hasilnya map kosong.
func InjectHeaders(ctx context.Context) map[string]string {
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	return carrier
}

// ExtractContext membaca konteks tracing dari header pesan dan menempelkannya
// ke ctx, sehingga span yang dibuat setelahnya menjadi kelanjutan trace asal.
func ExtractContext(ctx context.Context, headers map[string]string) context.Context {
	return otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(headers))
}

// SetupPropagation memasang format propagasi W3C Trace Context (header
// "traceparent") dan Baggage secara global. Dipanggil saat startup, walau
// tracing dimatikan, supaya konteks trace dari layanan lain tetap diteruskan.
func SetupPropagation() {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))
}
