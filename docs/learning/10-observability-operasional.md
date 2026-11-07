# 10. Observability dan Operasional

Backend yang tidak bisa diamati sama dengan backend yang tidak bisa
diperbaiki. Tiga pilarnya adalah **log**, **metrik**, dan **trace**.

## Log terstruktur dengan `log/slog`

`slog` adalah logger terstruktur bawaan Go (sejak 1.21):

```go
log.InfoContext(ctx, "http request", "method", "GET", "status", 200, "duration_ms", 12)
// {"time":"...","level":"INFO","msg":"http request","method":"GET","status":200,
//  "duration_ms":12,"request_id":"6c1b...","user_id":"9d80...","trace_id":"1cf3..."}
```

- **Key-value**, bukan kalimat bebas, sehingga mudah dicari di Loki, ELK,
  atau CloudWatch.
- `request_id`, `user_id`, dan `trace_id` ditambahkan otomatis oleh
  `observability.ContextHandler` dari context.
- Format JSON untuk production, format teks untuk development (`log.format`).
- Level: debug untuk detail pengembangan, info untuk kejadian normal, warn
  untuk hal janggal yang masih bisa diatasi, error untuk kegagalan yang
  perlu perhatian.

## Metrik Prometheus

Metrik menjawab pertanyaan "seberapa banyak" dan "seberapa cepat":

```go
// internal/observability/metrics.go
HTTPRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
    Name: "http_requests_total",
}, []string{"method", "route", "status"}),
```

- **Counter**: hanya naik (jumlah request).
- **Histogram**: distribusi nilai (durasi request, untuk p95/p99).
- **Gauge**: bisa naik dan turun (`outbox_pending`).

**Aturan kardinalitas**: nilai label harus berasal dari himpunan kecil.
Gunakan template route `/api/v1/contacts/:contactId`, bukan path
mentah `/api/v1/contacts/6f1c...`. Setiap nilai label baru menjadi time
series baru, dan bisa menghabiskan memori Prometheus.

Metrik hanya disajikan di **port admin** (9090/9091), bukan di port API publik.

## Distributed tracing (OpenTelemetry)

Trace menunjukkan perjalanan satu request lintas komponen:

```
POST /api/v1/contacts                    120ms
├── INSERT contacts                        3ms
├── INSERT audit_logs                      2ms
├── INSERT outbox_events                   2ms
└── (worker) consume contacts             15ms   ← trace yang sama!
    └── INSERT processed_events            2ms
```

Trace bisa bersambung sampai worker karena konteks trace (`traceparent`)
disimpan di header event outbox (`observability.InjectHeaders`), lalu
dibaca lagi oleh consumer (`observability.ExtractContext`).

Instrumentasi yang dipasang: Fiber, gRPC, GORM, dan Redis. Tracing diaktifkan
dengan `otel.enabled`. Di mode lengkap docker-compose, lihat hasilnya di
Jaeger (<http://localhost:16686>).

## Health check

`internal/delivery/admin/server.go`:

- **`/health/live`**: "prosesnya hidup". Tidak mengecek dependency, karena
  me-restart aplikasi tidak akan memperbaiki database yang mati.
- **`/health/ready`**: "siap menerima traffic". Mengecek database, Redis,
  dan storage. Mengembalikan 503 saat shutdown dimulai, supaya load balancer
  berhenti mengirim request baru.

Di Kubernetes, keduanya dipasang sebagai `livenessProbe` dan `readinessProbe`.

## Graceful shutdown

Urutannya (lihat [bab 03](03-concurrency-dan-context.md)):

1. Sinyal SIGTERM diterima.
2. Readiness menjadi 503.
3. Server berhenti menerima koneksi baru dan menunggu request yang sedang
   berjalan, dengan batas waktu.
4. Koneksi ke Redis dan database ditutup, lalu sisa trace dikirim.

Shutdown gRPC juga diberi batas waktu (`deliverygrpc.Shutdown`), karena
stream yang tidak pernah selesai bisa menahan proses selamanya.

## Konfigurasi dan secret

- `config.json` berisi default yang aman dibagikan.
- Secret hanya lewat environment variable (`APP_JWT_SECRET`,
  `APP_DATABASE_PASSWORD`).
- Mode production divalidasi ketat: aplikasi menolak start bila konfigurasi
  tidak aman.

## Docker dan docker-compose

`Dockerfile` memakai **multi-stage build**:

1. Tahap build: image `golang:1.27.3`, kompilasi binary statis (`CGO_ENABLED=0`).
2. Tahap runtime: `distroless/static:nonroot`, tanpa shell dan tanpa package
   manager, berjalan sebagai user non-root.

Hasilnya image kecil dengan permukaan serangan minimal.

`docker-compose.yml` punya dua mode:

- **Default**: mysql + migrate + web + worker.
- **Profil `full`**: ditambah Kafka, Redis, S3 (SeaweedFS), Mailpit, Jaeger,
  dan Prometheus. Fitur diaktifkan lewat `deploy/full.env`.

## CI

`.github/workflows/ci.yml` menjalankan lint, unit test dengan race detector,
integration test, govulncheck, dan build Docker di setiap push dan pull
request. Dependabot memperbarui dependency setiap minggu.

## Latihan

1. Tambahkan metrik `contacts_created_total` (counter) yang naik di
   `ContactUseCase.Create`. Bagaimana menambahkannya tanpa membuat use case
   bergantung pada Prometheus? (Petunjuk: port baru, atau hook di delivery.)
2. Jalankan mode lengkap, buat kontak, lalu cari trace-nya di Jaeger.
   Temukan span dari worker yang menjadi bagian trace yang sama.
3. Tambahkan pre-stop delay yang bisa dikonfigurasi (`shutdown.drain_delay`)
   antara readiness 503 dan penghentian server. Mengapa ini membantu di
   Kubernetes?
