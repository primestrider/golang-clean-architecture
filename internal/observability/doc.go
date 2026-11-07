// Package observability berisi perangkat untuk "melihat ke dalam" aplikasi
// yang sedang berjalan: logging terstruktur yang sadar context, metrik
// Prometheus, dan helper tracing OpenTelemetry.
//
// Ide utamanya: informasi korelasi (request id, user id, trace id) disimpan
// di context.Context sekali saja, lalu otomatis ikut di setiap baris log,
// sehingga satu request bisa ditelusuri dari awal sampai akhir.
package observability
