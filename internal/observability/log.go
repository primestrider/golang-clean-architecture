package observability

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

// ctxKey adalah tipe kunci privat untuk context, supaya tidak bertabrakan
// dengan kunci dari package lain.
type ctxKey int

const (
	requestIDKey ctxKey = iota
	userIDKey
)

// WithRequestID mengembalikan context baru yang membawa request id.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// RequestID mengambil request id dari context, atau string kosong.
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// WithUserID mengembalikan context baru yang membawa id user yang login.
func WithUserID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, userIDKey, id)
}

// UserID mengambil id user yang login dari context, atau string kosong.
func UserID(ctx context.Context) string {
	id, _ := ctx.Value(userIDKey).(string)
	return id
}

// ContextHandler adalah slog.Handler pembungkus yang menambahkan request_id,
// user_id, trace_id, dan span_id dari context ke setiap record log.
//
// Karena itu kode lain cukup memanggil logger.InfoContext(ctx, ...) tanpa
// perlu menyebut id-id tersebut satu per satu.
type ContextHandler struct {
	next slog.Handler
}

// NewContextHandler membungkus handler slog lain (JSON atau text).
func NewContextHandler(next slog.Handler) slog.Handler {
	return &ContextHandler{next: next}
}

// Enabled meneruskan pengecekan level ke handler yang dibungkus.
func (h *ContextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

// Handle menambahkan atribut korelasi dari context, lalu meneruskan record.
// Atribut yang kosong tidak ditulis supaya log tetap ringkas.
func (h *ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	if id := RequestID(ctx); id != "" {
		r.AddAttrs(slog.String("request_id", id))
	}
	if id := UserID(ctx); id != "" {
		r.AddAttrs(slog.String("user_id", id))
	}
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		r.AddAttrs(slog.String("trace_id", sc.TraceID().String()), slog.String("span_id", sc.SpanID().String()))
	}
	return h.next.Handle(ctx, r)
}

// WithAttrs menjaga agar logger turunan (logger.With(...)) tetap memperkaya
// log dari context.
func (h *ContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ContextHandler{next: h.next.WithAttrs(attrs)}
}

// WithGroup menjaga perilaku yang sama untuk logger.WithGroup(...).
func (h *ContextHandler) WithGroup(name string) slog.Handler {
	return &ContextHandler{next: h.next.WithGroup(name)}
}
