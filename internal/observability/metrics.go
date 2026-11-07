package observability

import (
	"errors"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

// Metrics berisi semua metrik Prometheus milik aplikasi.
//
// Aturan penting untuk label: nilainya harus berasal dari himpunan kecil
// yang tetap (method, template route, status). Jangan pernah memakai nilai
// bebas seperti path mentah atau id user sebagai label, karena setiap nilai
// baru membuat time series baru dan bisa menghabiskan memori Prometheus.
type Metrics struct {
	HTTPRequests *prometheus.CounterVec
	HTTPDuration *prometheus.HistogramVec

	OutboxPublished       prometheus.Counter
	OutboxPublishFailures prometheus.Counter

	MessagesProcessed *prometheus.CounterVec
	MessagesDLQ       *prometheus.CounterVec

	registry prometheus.Registerer
}

// NewMetrics membuat dan mendaftarkan metrik ke registry, termasuk metrik
// runtime Go (goroutine, memori, GC) dan proses (CPU, file descriptor).
func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		HTTPRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Jumlah request HTTP per method, template route, dan status.",
		}, []string{"method", "route", "status"}),
		HTTPDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Lama pemrosesan request HTTP.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "route"}),
		OutboxPublished: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "outbox_published_total",
			Help: "Jumlah event outbox yang berhasil dikirim.",
		}),
		OutboxPublishFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "outbox_publish_failures_total",
			Help: "Jumlah percobaan kirim event outbox yang gagal.",
		}),
		MessagesProcessed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "messages_processed_total",
			Help: "Jumlah pesan yang diproses consumer per topik dan hasil.",
		}, []string{"topic", "result"}),
		MessagesDLQ: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "messages_dlq_total",
			Help: "Jumlah pesan yang dipindah ke dead-letter queue per topik.",
		}, []string{"topic"}),
		registry: reg,
	}
	reg.MustRegister(
		m.HTTPRequests, m.HTTPDuration,
		m.OutboxPublished, m.OutboxPublishFailures,
		m.MessagesProcessed, m.MessagesDLQ,
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	return m
}

// Register mendaftarkan collector tambahan, misalnya statistik pool
// database atau gauge outbox.
func (m *Metrics) Register(cs ...prometheus.Collector) {
	m.registry.MustRegister(cs...)
}

// HTTPMiddleware mencatat jumlah dan durasi request. Label route memakai
// template route Fiber (misalnya "/api/v1/contacts/:contactId"), bukan path
// mentah, supaya jumlah time series tetap kecil. Request ke route yang tidak
// ada diberi label "unmatched".
func (m *Metrics) HTTPMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()
		err := c.Next()

		// Di stack aplikasi, error sudah dirender oleh middleware di bawahnya
		// sehingga status respons sudah final. Bila belum (misalnya error
		// bawaan Fiber yang langsung sampai ke sini), ambil status dari error.
		status := c.Response().StatusCode()
		if fe, ok := errors.AsType[*fiber.Error](err); ok {
			status = fe.Code
		}
		route := c.Route().Path
		if status == fiber.StatusNotFound && !isRegisteredRoute(c) {
			route = "unmatched"
		}
		m.HTTPRequests.WithLabelValues(c.Method(), route, strconv.Itoa(status)).Inc()
		m.HTTPDuration.WithLabelValues(c.Method(), route).Observe(time.Since(start).Seconds())
		return err
	}
}

// isRegisteredRoute melaporkan apakah request cocok dengan route handler
// (bukan hanya middleware global yang terpasang di semua path).
func isRegisteredRoute(c fiber.Ctx) bool {
	r := c.Route()
	return r != nil && r.Path != "/" && r.Path != "/*"
}
