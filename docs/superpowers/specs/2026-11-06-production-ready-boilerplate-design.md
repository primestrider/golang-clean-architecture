# Desain: Boilerplate Go Clean Architecture Siap Produksi

Tanggal: 2026-11-06
Branch dasar: `upgrade-go-1.27.3` (Go 1.27.3, dependensi sudah di-upgrade)

## 1. Maksud dan alasan

Repository ini adalah template Go Clean Architecture. Isinya Fiber, GORM/MySQL, Kafka, dan domain demo user → contact → address. Pemilik akan memakainya untuk **dua hal sekaligus**:

1. **Dasar API untuk aplikasi-aplikasinya sendiri.** Template harus siap produksi: aman, bisa diamati, bisa diuji, dan mudah di-deploy.
2. **Bahan belajar arsitektur backend Go.** Setiap bagian harus mudah dibaca dan dijelaskan lewat komentar.

Template saat ini baru menyediakan fondasi (layering, CRUD, transaksi, validasi, auth token sederhana, event Kafka). Desain ini menambahkan semua aspek backend yang biasanya dibutuhkan aplikasi produksi.

## 2. Tujuan, non-tujuan, anti-tujuan

### Tujuan
- Mencakup enam kelompok: (A) kesiapan produksi, (B) keamanan, (C) keandalan data & event, (D) testability & kualitas, (E) observability & tooling, (F) fitur backend umum.
- **Setiap package, tipe, fungsi, dan method yang diekspor, serta logika non-trivial yang tidak diekspor, punya doc comment gaya godoc dalam Bahasa Indonesia.** Formatnya `// NamaSimbol ...` tepat di atas deklarasi. Ini padanan JSDoc di Go. Komentar menjelaskan *apa* dan *mengapa*, bukan mengulang kode. Setiap package punya `doc.go` yang menjelaskan peran layer-nya dalam arsitektur.
- Domain demo (user, contact, address) tetap dipakai sebagai contoh untuk mendemokan semua fitur.
- Best practice dan keamanan diutamakan (permintaan eksplisit pemilik).

### Non-tujuan
- Verifikasi email, reset password, OAuth/social login, 2FA.
- Multi-tenancy.
- Manifest Kubernetes/Helm, dashboard Grafana, frontend/admin UI.
- Kompatibilitas dengan API lama. Belum ada client, jadi format auth, error, dan respons boleh diubah bebas.

### Anti-tujuan (gagal walaupun "berfungsi")
- "Magic" yang menyulitkan belajar, misalnya DI container (wire/fx) atau framework tersembunyi. Wiring dependency tetap manual dan eksplisit di `internal/config` (bootstrap).
- Infrastruktur wajib berlebihan. **Mode minimal hanya butuh MySQL.** Kafka, Redis, MinIO, SMTP, dan OTel bisa dimatikan lewat config, dan aplikasi tetap jalan dengan perilaku pengganti yang terdokumentasi.
- Secret di repository, log yang membocorkan data sensitif (password, token, hash), atau pesan error yang membocorkan detail internal ke client.
- Komentar yang hanya mengulang nama fungsi.

## 3. Batasan

- Go 1.27.3. Platform pengembangan pemilik adalah Windows (PowerShell/Git Bash). Docker diasumsikan tersedia.
- MySQL 8 (dibutuhkan untuk `SELECT ... FOR UPDATE SKIP LOCKED` di outbox relay).
- Identifier kode tetap berbahasa Inggris. Komentar dan README berbahasa Indonesia.
- Pekerjaan dikerjakan **bertahap dengan commit terpisah per fase** (lihat §6), berurutan dalam satu branch kerja tanpa berhenti untuk review per fase. Pemilik mereview di akhir. Setiap fase harus lolos build, lint, dan test sebelum lanjut.
- **Module path** diganti dari `golang-clean-architecture` menjadi `github.com/primestrider/golang-clean-architecture` di awal fase 1.

## 4. Keputusan teknologi (default paling umum, keputusan saya)

Pemilik menyerahkan pilihan teknologi. Semua di bawah ini adalah **keputusan saya**, dengan alasannya:

| Kebutuhan | Pilihan | Alasan |
|---|---|---|
| HTTP framework | **Fiber v3** (v3.6.x) | Proyek sudah memakai Fiber. v3 sudah stabil dan merupakan versi aktif untuk proyek baru. Tidak ada client, jadi migrasi API aman. |
| Logging | **`log/slog`** (stdlib), gantikan logrus | Standar bawaan Go, terstruktur, tanpa dependensi. Logrus dalam mode maintenance. |
| Config | **Viper** (tetap) + override env var berprefix `APP_` | Sudah dipakai. Struct `Config` bertipe dan divalidasi saat startup (fail fast). |
| JWT | **golang-jwt/jwt v5**, HS256 | Library JWT paling umum di Go. HS256 cukup untuk satu layanan. Catatan di doc comment: pindah ke RS256/EdDSA jika token diverifikasi layanan lain. |
| Hash password | bcrypt (tetap) | Sudah dipakai, standar. |
| Redis | **redis/go-redis v9** | Client resmi. |
| Object storage | **minio-go v7** | Kompatibel dengan S3 (AWS S3, MinIO, R2). API lebih sederhana daripada aws-sdk-go-v2. |
| Scheduler | **go-co-op/gocron v2** | Aktif dipelihara dan mendukung distributed locking. robfig/cron tidak lagi dipelihara. |
| Email | **wneessen/go-mail** via SMTP; **Mailpit** di docker-compose untuk dev | SMTP universal (bisa ke SES, SendGrid, Mailgun). net/smtp stdlib sudah frozen. |
| gRPC | **google.golang.org/grpc** + **buf** untuk codegen; interceptor dari **grpc-ecosystem/go-grpc-middleware v2** | Standar de facto. Kode hasil generate di-commit, jadi pembaca tidak wajib memasang buf. |
| Metrics | **prometheus/client_golang** | Standar. |
| Tracing | **OpenTelemetry** + exporter OTLP; **Jaeger** di compose | Standar vendor-neutral. |
| Migrasi | **golang-migrate** (tetap); juga dipakai sebagai library di integration test | Sudah dipakai. |
| Mock | **mockery** (berbasis testify/mock) | testify sudah dipakai di proyek. |
| Integration test | **testcontainers-go** (modul MySQL, Redis, Kafka) | Test tidak bergantung pada DB lokal. |
| Lint / keamanan | **golangci-lint** v2, **govulncheck** | Standar. |
| CI | **GitHub Actions** + Dependabot | Paling umum. |
| Build lokal | **Makefile** (dijalankan via Git Bash/WSL) | Paling umum. README juga mencantumkan perintah mentahnya untuk pengguna PowerShell. |
| Dokumentasi API | **OpenAPI 3.1 ditulis tangan** (`api/openapi.yaml`) + **Swagger UI** | Spec ditulis tangan menghindari anotasi swag di setiap handler (kode lebih bersih untuk dipelajari). Spec lama `api/api-spec.json` diganti. |

## 5. Desain per area

### 5.0 Fondasi arsitektur (refactor yang dibutuhkan fitur lain)

**Use case tidak lagi bergantung pada Fiber.** Saat ini use case mengembalikan `fiber.ErrXxx`, padahal nanti ada delivery gRPC. Dibuat package `internal/apperror` berisi error domain bertipe: `Code` (mis. `VALIDATION_FAILED`, `NOT_FOUND`, `CONFLICT`, `UNAUTHORIZED`, `FORBIDDEN`, `RATE_LIMITED`, `INTERNAL`), pesan aman untuk client, detail per-field opsional, dan error penyebab (hanya untuk log). Delivery HTTP memetakannya ke status HTTP. Delivery gRPC memetakannya ke `codes.Code`.

**Transaksi lewat context (pola Transactor).** Interface `Transactor.WithinTransaction(ctx, func(ctx) error) error` menyimpan `*gorm.DB` transaksi di context. Repository mengambil DB dari context (`dbFrom(ctx)`), dan memakai DB biasa jika tidak ada transaksi. Dengan begitu:
- Use case tidak menyentuh `*gorm.DB` dan bisa di-unit-test dengan mock.
- Operasi baca tidak perlu membuka transaksi (memperbaiki overhead yang ada sekarang).

**Interface didefinisikan di sisi pemakai** (package `usecase`): `UserRepository`, `ContactRepository`, `AddressRepository`, `RefreshTokenRepository`, `OutboxRepository`, `AuditRepository`, `Cache`, `ObjectStorage`, dll. Implementasi konkret tetap di `repository/` dan `gateway/`. `Repository[T]` generik tetap dipakai sebagai embedding.

**Struktur binary:**
- `cmd/web`: API HTTP (Fiber), gRPC (port terpisah, opsional), dan admin server.
- `cmd/worker`: consumer Kafka, outbox relay, scheduler, dan admin server.
- **Admin server** (`net/http`, port terpisah, default 9090 web / 9091 worker): `/health/live`, `/health/ready`, `/metrics`. Dipisah dari port publik supaya metrics tidak terekspos ke internet.

**Versi API:** semua route di bawah `/api/v1`.

**Graceful shutdown (kedua binary):** `signal.NotifyContext(SIGINT, SIGTERM)`. Urutannya: tandai readiness = false → hentikan penerimaan request/pesan baru (Fiber `ShutdownWithTimeout`, gRPC `GracefulStop`, cancel consumer & relay & scheduler) → tunggu yang sedang berjalan selesai (`sync.WaitGroup`, batas waktu dari config, default 15 detik) → tutup producer Kafka, Redis, MinIO, DB → flush OTel. `time.Sleep` dan `SIGKILL` di worker dihapus.

**Config:** `config.json` tetap menyimpan default non-rahasia. Setiap key bisa di-override env var (`APP_DATABASE_PASSWORD`, `APP_JWT_SECRET`, ...). `.env.example` dibuat dan `.env` masuk `.gitignore`. Saat startup config di-load ke struct bertipe dan divalidasi. Jika `app.env=production`: JWT secret wajib ≥ 32 byte, Swagger UI mati, CORS tidak boleh `*`. Aplikasi menolak start jika aturan ini dilanggar.

### 5.1 (A) Kesiapan produksi

- **Middleware HTTP** (urutan): recover (panic → 500 tanpa stack ke client, stack masuk log) → requestid (`X-Request-ID`, diterima dari client jika valid atau dibuat baru) → OTel → metrics → access log (slog: method, path, status, latensi, request_id, user_id) → helmet (header keamanan) → CORS (allowlist dari config) → limiter global → timeout per request (default 10 detik) → body limit (default 1 MB; khusus route upload 2 MB).
- **Format error: RFC 9457 Problem Details** (`application/problem+json`):
  ```json
  {"type":"https://example.com/problems/validation-failed","title":"Validation failed","status":400,
   "detail":"...","instance":"/api/v1/contacts","request_id":"...","errors":[{"field":"email","message":"must be a valid email"}]}
  ```
  Respons sukses tetap memakai envelope `{"data": ..., "paging": ...}`. Pesan validasi dibuat dari tag validator dalam Bahasa Inggris yang sederhana. Error 500 tidak pernah menyertakan detail internal.
- **Health:** `live` selalu 200 selama proses hidup. `ready` mengecek DB ping dan dependensi yang diaktifkan (Redis, Kafka producer). Mengembalikan 503 saat shutdown.

### 5.2 (B) Keamanan

**Perubahan skema user** (bebas karena belum ada client): `id` UUID (sebelumnya username pilihan user), `email` unik (dipakai login dan untuk fitur email), `name`, `password` (bcrypt), `role`, `avatar_key`, timestamps, `deleted_at`, `version`. Kolom `token` dihapus.

**Auth:**
- **Access token:** JWT HS256, umur 15 menit (config). Claims: `sub` (user id), `role`, `iss`, `aud`, `iat`, `exp`, `jti`. Verifikasi wajib mengecek algoritma, `iss`, `aud`, dan `exp`. Dikirim lewat header `Authorization: Bearer <token>`.
- **Refresh token:** string acak 32 byte (crypto/rand, base64url), umur 7 hari (config). **Disimpan hanya dalam bentuk hash SHA-256** di tabel `refresh_tokens` (`id`, `user_id`, `family_id`, `token_hash` unik, `expires_at`, `revoked_at`, `replaced_by_id`, `created_at`, `user_agent`, `ip`).
- **Rotasi:** `POST /api/v1/auth/refresh` mencabut token lama dan menerbitkan pasangan baru dalam keluarga yang sama. **Deteksi reuse:** jika token yang sudah dicabut dipakai lagi, seluruh keluarga dicabut (indikasi pencurian), lalu dicatat ke log dan audit.
- Token dikembalikan di body JSON (cocok untuk mobile dan API client). Cookie tidak dipakai (keputusan saya, karena konsumennya API).
- **Logout** (`POST /api/v1/auth/logout`) mencabut keluarga refresh token. Access token tetap berlaku sampai kedaluwarsa (maksimal 15 menit). Ini trade-off yang didokumentasikan; denylist tidak dibuat.
- **Login:** pesan error seragam untuk email tidak ditemukan dan password salah. Saat user tidak ditemukan, bcrypt tetap dijalankan terhadap hash dummy supaya waktu respons tidak membocorkan keberadaan email.
- **Password:** minimal 8 karakter, maksimal 72 byte (batas bcrypt).
- **Ganti password** (lewat update profil) wajib menyertakan password lama dan mencabut semua refresh token user.

**Rate limiting:** global 100 request/menit per IP. Endpoint `login`, `register`, dan `refresh`: 5 request/menit per kombinasi IP+email (refresh: per IP). Storage-nya Redis jika aktif (berlaku lintas instance), jika tidak memakai memory (per instance, terdokumentasi). Respons 429 menyertakan header `Retry-After`. Config `server.trusted_proxies` dipakai supaya IP client terbaca benar di belakang load balancer.

**RBAC:** kolom `role` (`user` | `admin`), ikut dalam claim JWT. Middleware `RequireRole("admin")`. Endpoint admin demo:
- `GET /api/v1/admin/users` (daftar user, dengan paging)
- `PATCH /api/v1/admin/users/:id/role`

Admin pertama dibuat lewat target `make create-admin EMAIL=...` (perintah kecil di `cmd/cli`, atau SQL terdokumentasi, terserah builder). Ownership tetap dicek: user hanya bisa mengakses contact/address miliknya.

### 5.3 (C) Keandalan data dan event

**Migrasi:** karena belum pernah di-deploy, tiga migrasi lama **ditulis ulang** menjadi set migrasi bersih yang mudah dibaca (keputusan saya). Komentar di README menegaskan bahwa di sistem yang sudah berjalan, migrasi lama tidak boleh diubah.

**Soft delete:** user, contact, address memakai `gorm.io/plugin/soft_delete` dengan `deleted_at BIGINT NOT NULL DEFAULT 0` (milidetik, konsisten dengan timestamp lain). Nilai 0 berarti aktif, sehingga unique index `(email, deleted_at)` tetap bekerja. Menghapus contact juga men-soft-delete address-nya dalam transaksi yang sama.

**Optimistic locking:** kolom `version INT NOT NULL DEFAULT 1` di users, contacts, addresses. Request update (PUT/PATCH) wajib menyertakan `version`. Repository menjalankan `UPDATE ... SET version = version + 1 WHERE id = ? AND version = ?`. Jika 0 baris berubah, use case mengembalikan `CONFLICT` (409). `version` ikut dalam respons.

**Audit trail:** tabel `audit_logs` (`id`, `actor_id`, `action` seperti `contact.updated`, `entity_type`, `entity_id`, `request_id`, `changes` JSON berisi field yang berubah before/after, `created_at`). Ditulis use case **dalam transaksi yang sama** dengan perubahan data. Field sensitif (password, hash token) tidak pernah dicatat.

**Event envelope** (gantikan event per-entity yang sekarang):
```json
{"event_id":"uuid","event_type":"contact.created","occurred_at":"RFC3339","aggregate_id":"...","actor_id":"...","data":{...}}
```
Key Kafka = `aggregate_id` (menjaga urutan per entity). Topik: `users`, `contacts`, `addresses`. Payload tidak boleh berisi password atau token.

**Transactional outbox:** tabel `outbox_events` (`id` = event_id, `topic`, `key`, `event_type`, `payload` JSON, `headers` JSON termasuk trace context, `created_at`, `published_at` NULL, `attempts`, `last_error`). Use case menulis event ke outbox **di dalam transaksi bisnis**. Publish langsung dari use case dihapus.
- **Relay** di `cmd/worker`: **hanya satu relay aktif** di antara semua replica, lewat MySQL `GET_LOCK('outbox_relay')`. Ini tidak butuh Redis, dan urutan per entity terjaga. Replica lain menunggu dan mengambil alih jika pemegang lock mati. Polling setiap 1 detik (config), batch 100, urut `created_at, id`, lalu set `published_at`. Jaminannya **at-least-once**, dengan **urutan terjaga per `aggregate_id`**:
  - Jika satu event gagal, event berikutnya dengan `aggregate_id` yang sama tidak dikirim di batch itu. Event itu dicoba lagi di polling berikutnya dengan `attempts++` dan `last_error` tersimpan.
  - Setelah `max_attempts` (default 10), status event menjadi `failed` (diparkir). Event yang diparkir **memblok** event lain untuk aggregate yang sama, terlihat di metrik `outbox_failed`, dan dicatat di log error. Operator bisa memulihkannya dengan me-reset `attempts`.
  - Kolom tambahan `status` (`pending` | `published` | `failed`).
  - (Keputusan saya.)
- **Outbox selalu ditulis**, termasuk saat Kafka mati. Relay punya dua mode:
  - **Kafka aktif:** event dikirim ke Kafka, lalu consumer di worker memprosesnya.
  - **Kafka mati:** relay mengirim event **langsung ke handler lokal** (registry handler yang sama dengan yang dipakai consumer Kafka, termasuk idempotency `processed_events`).
  - Dengan begitu email selamat datang tetap terkirim di mode minimal (hanya MySQL, tetapi worker tetap dijalankan). Satu jalur kode, tanpa implementasi kedua. (Keputusan saya.)

**Consumer (worker):**
- **Idempotency:** tabel `processed_events` (`consumer_name`, `event_id`, `processed_at`, PK gabungan). Handler memeriksa dan mencatat dalam transaksi yang sama dengan efek sampingnya. Event duplikat dilewati.
- **Retry dan DLQ:** retry di dalam proses maksimal 3 kali dengan backoff eksponensial (config). Setelah itu pesan dikirim ke topik `<topik>.dlq` dengan header `x-error`, `x-original-topic`, `x-attempts`. Setelah itu offset di-commit supaya partisi tidak macet.
- Handler demo: user consumer mengirim email selamat datang saat `user.registered` (lihat §5.6). Handler lain mencatat log terstruktur.

### 5.4 (D) Testability dan kualitas

- **Unit test** untuk setiap use case dengan mock mockery (`internal/usecase/*_test.go`). Mencakup jalur sukses dan error utama: validasi, not found, conflict versi, reuse refresh token, rate limit tidak di sini.
- **Integration test** di `test/` dengan build tag `integration`, memakai testcontainers (MySQL; Redis dan Kafka untuk test yang membutuhkannya). Migrasi dijalankan lewat golang-migrate sebagai library. **Setiap test independen**: data disiapkan sendiri, tidak saling memanggil test lain.
- Perintah: `make test` (unit), `make test-integration` (butuh Docker).
- **golangci-lint v2** dengan konfigurasi `.golangci.yml` (minimal: govet, staticcheck, errcheck, gosec, revive dengan aturan doc comment exported, bodyclose, sqlclosecheck, errorlint, gocritic).
- **GitHub Actions** (`.github/workflows/ci.yml`): lint → unit test (dengan `-race` dan coverage) → integration test → govulncheck → docker build. **Dependabot** untuk gomod, GitHub Actions, dan Docker.

### 5.5 (E) Observability dan tooling

- **slog:** JSON di production, text di development (config). Handler kustom menambahkan `request_id`, `trace_id`, `span_id`, dan `user_id` dari context. Logger GORM diarahkan ke slog (query lambat > 200 ms di level warn, query biasa di level debug).
- **Prometheus:** metrik HTTP (`http_requests_total`, `http_request_duration_seconds` dengan label route template, bukan path mentah), metrik gRPC, statistik pool DB, metrik outbox (`outbox_pending`, `outbox_publish_failures_total`), metrik consumer (`messages_processed_total`, `messages_dlq_total`), serta metrik runtime Go.
- **OpenTelemetry tracing:** Fiber, gRPC (otelgrpc), GORM (`gorm.io/plugin/opentelemetry`), Redis (redisotel), HTTP client. Trace context Kafka dipropagasikan manual lewat header pesan (via outbox `headers`), sehingga trace bersambung dari request HTTP ke consumer. Sampling ratio diatur lewat config. Default **mati**.
- **Dockerfile** multi-stage: build dengan `golang:1.27.3`, runtime `gcr.io/distroless/static:nonroot`. Satu image berisi kedua binary, dipilih lewat command.
- **docker-compose.yml:**
  - Profil default: mysql, migrate, web, worker.
  - Profil `full`: tambah kafka (KRaft), redis, minio (+ pembuatan bucket), mailpit, jaeger, prometheus.
- **Makefile:** `run-web`, `run-worker`, `build`, `test`, `test-integration`, `lint`, `vuln`, `migrate-up`, `migrate-down`, `migrate-create`, `mocks`, `proto`, `compose-up`, `compose-full`, `compose-down`, `create-admin`.

### 5.6 (F) Fitur backend umum

Semua bersifat opsional lewat config, dengan perilaku pengganti saat mati:

- **Redis cache** (cache-aside) untuk `GET /contacts/:id`. Key mencakup user id, TTL 5 menit. Cache dihapus setelah commit pada update/delete. Redis juga dipakai untuk storage rate limiter dan lock scheduler. *Jika mati:* tanpa cache, limiter di memory, scheduler tanpa lock (diasumsikan satu replica worker, terdokumentasi).
- **Scheduler** (di worker, gocron v2, distributed lock Redis):
  - Hapus refresh token kedaluwarsa atau dicabut lebih dari 30 hari (harian).
  - Hapus outbox yang sudah terpublikasi lebih dari 7 hari dan `processed_events` lebih dari 30 hari (harian).
  - Jadwal diatur lewat config.
- **Upload avatar ke S3/MinIO:**
  - `PUT /api/v1/users/_current/avatar` (multipart, field `file`, maksimal 2 MB).
  - Tipe file dideteksi dari isi (`http.DetectContentType`), bukan dari header atau ekstensi. Hanya JPEG, PNG, dan WebP.
  - Object key: `avatars/{userId}/{uuid}.{ext}`. Avatar lama dihapus setelah commit berhasil.
  - Respons user berisi `avatar_url` berupa **presigned URL** GET (berlaku 15 menit). Bucket tetap privat.
  - `DELETE /api/v1/users/_current/avatar` untuk menghapus avatar.
  - *Jika mati:* route tidak didaftarkan dan `avatar_url` tidak muncul.
- **Email:** `gateway/email` (interface `Mailer`, implementasi SMTP via go-mail, template `html/template` yang di-embed). Worker mengirim email selamat datang saat event `user.registered` (memakai consumer idempoten). *Jika mati:* memakai mailer log-only.
- **Swagger UI:**
  - `GET /docs` menyajikan Swagger UI (aset dari CDN dengan versi dipatok).
  - `GET /openapi.yaml` menyajikan spec yang di-embed (`go:embed`).
  - Spec mencakup semua endpoint, skema error Problem Details, dan security scheme Bearer.
  - Mati otomatis di production.
- **gRPC:**
  - `api/proto/contact/v1/contact.proto` berisi `ContactService` (Create, Get, Update, Delete, List) yang memakai use case yang sama dengan HTTP. Ini mendemonstrasikan bahwa use case tidak terikat ke HTTP.
  - Auth: access JWT di metadata `authorization: Bearer ...`.
  - Interceptor: recovery, request id, logging, auth, OTel, Prometheus, validasi.
  - Termasuk `grpc.health.v1`. Reflection hanya aktif di development.
  - Port default 50051. *Jika mati:* tidak di-listen.
  - Kode hasil generate di `internal/delivery/grpc/pb/` dan di-commit. `buf.yaml` dan `buf.gen.yaml` disediakan.

## 6. Urutan fase (satu atau lebih commit per fase, di branch kerja dari `upgrade-go-1.27.3`)

1. **Fondasi:** Fiber v3, slog, config bertipe + env, apperror + Problem Details + error validasi per field, Transactor + interface repository, `/api/v1`, graceful shutdown, admin server + health, doc comment untuk semua kode yang ada.
2. **Testing & CI:** mockery + unit test use case, testcontainers integration test, golangci-lint, GitHub Actions, govulncheck, Dependabot. (Dikerjakan awal supaya fase berikutnya langsung disertai test.)
3. **Keamanan:** skema user baru, JWT + refresh rotation, middleware keamanan, rate limit, RBAC + endpoint admin.
4. **Keandalan data:** soft delete, optimistic locking, audit log, event envelope, outbox + relay, idempotency + DLQ.
5. **Observability & tooling:** metrik Prometheus, OTel, Dockerfile, docker-compose, Makefile.
6. **Fitur:** Redis cache/limiter/lock, scheduler, upload avatar, email, Swagger UI + OpenAPI, gRPC.
7. **Penutup:** README lengkap (arsitektur, cara menjalankan mode minimal dan full, penjelasan setiap fitur dan trade-off-nya), `test/manual.http` diperbarui.

Setiap fase selesai jika `go build ./...`, `golangci-lint run`, unit test, dan integration test lolos.

## 7. Diserahkan ke builder

- Struktur file di dalam setiap layer, nama package helper, dan pembagian file.
- Pesan validasi persisnya, URL `type` Problem Details, dan nama metrik selain yang disebut.
- Versi persis library (ambil versi stabil terbaru saat pengerjaan).
- Bentuk perintah `create-admin` (CLI kecil atau SQL).
- Detail konfigurasi golangci-lint di luar linter minimal yang disebut.
- Isi template email.
- Detail kecil lain yang bisa diputuskan builder dan cukup didokumentasikan:
  - apakah `PATCH /admin/users/:id/role` juga memakai `version`
  - grace period untuk deteksi reuse refresh token
  - endpoint publik MinIO untuk presigned URL
  - CSP Swagger UI terhadap helmet
  - Default saya: admin **tidak** bypass ownership contact/address.

## 8. Ringkasan keputusan yang saya ambil (untuk diperiksa pemilik)

1. Fiber v2 → **v3**, logrus → **slog**.
2. Error memakai **RFC 9457 Problem Details**. Respons sukses tetap `{data, paging}`.
3. User memakai **UUID + email** untuk login (sebelumnya username sebagai ID).
4. Refresh token di **body JSON**, bukan cookie. Logout tidak mencabut access token yang masih berlaku (maksimal 15 menit).
5. **Migrasi lama ditulis ulang** (belum pernah di-deploy).
6. `/metrics` dan health check di **port admin terpisah**.
7. OpenAPI **ditulis tangan**, bukan dari anotasi kode.
8. Komentar dan README dalam **Bahasa Indonesia**. Identifier tetap Bahasa Inggris.
