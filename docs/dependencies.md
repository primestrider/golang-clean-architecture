# Dependency Proyek: `go.mod` dan `go.sum`

Dokumen ini menjelaskan cara Go mengelola dependency di proyek ini, serta fungsi setiap package yang terpasang.

## Daftar Isi

- [Apa itu `go.mod`](#apa-itu-gomod)
- [Apa itu `go.sum`](#apa-itu-gosum)
- [Dependency langsung](#dependency-langsung)
- [Dependency tidak langsung (indirect)](#dependency-tidak-langsung-indirect)
- [Perintah yang sering dipakai](#perintah-yang-sering-dipakai)

---

## Apa itu `go.mod`

`go.mod` adalah file definisi **module** Go. Isinya identitas proyek dan daftar dependency beserta versinya. File ini ditulis manusia dan dirawat oleh perintah `go`.

```go
module github.com/primestrider/golang-clean-architecture  // nama module, prefix import

go 1.27.3                                                // versi minimum bahasa Go

require (                          // dependency langsung
	github.com/gofiber/fiber/v3 v3.6.1
	...
)

require (                          // dependency tidak langsung
	github.com/valyala/fasthttp v1.77.0 // indirect
	...
)
```

| Directive | Fungsi |
|---|---|
| `module` | Nama module. Package internal di-import dengan prefix ini, misalnya `github.com/primestrider/golang-clean-architecture/internal/config`. |
| `go` | Versi minimum Go untuk mem-build proyek. Nilai ini juga menentukan fitur bahasa yang boleh dipakai. Contohnya, `for b.Loop()` di benchmark butuh Go ≥ 1.24. |
| `require` | Daftar module beserta versi minimum yang dibutuhkan. |
| `// indirect` | Module ini tidak di-import langsung oleh kode proyek, tapi dibutuhkan oleh dependency lain. |

### Format versi

Go memakai [Semantic Versioning](https://semver.org) dengan format `vMAJOR.MINOR.PATCH`:

- **PATCH** (`v2.52.9` → `v2.52.16`): perbaikan bug atau keamanan, tanpa perubahan API.
- **MINOR** (`v1.30.1` → `v1.31.2`): fitur baru yang tetap kompatibel.
- **MAJOR** (`v2` → `v3`): ada breaking change. Untuk v2 ke atas, nomor major menjadi bagian dari path import, misalnya `github.com/gofiber/fiber/v2` dan `github.com/gofiber/fiber/v3`. Karena itu kedua versi dianggap module yang berbeda.
- **Pseudo-version** (`v0.0.0-20250401214520-65e299d6c5c9`): dipakai ketika module tidak punya tag rilis. Formatnya waktu commit (UTC) diikuti hash commit.

### Minimal Version Selection (MVS)

Jika beberapa dependency membutuhkan versi berbeda dari module yang sama, Go memilih **versi tertinggi di antara versi minimum yang diminta**. Go tidak otomatis memakai versi terbaru yang ada di internet. Akibatnya build bersifat *reproducible*: hasilnya sama selama `go.mod` tidak berubah.

---

## Apa itu `go.sum`

`go.sum` berisi **checksum kriptografis** untuk setiap module yang pernah diunduh. File ini dibuat otomatis dan **tidak boleh diedit manual**.

```
github.com/IBM/sarama v1.61.1 h1:I59MWPHQUWqJNdRpsDUcbeCriog8SxjQaPfHNWxidEg=
github.com/IBM/sarama v1.61.1/go.mod h1:dITlGHIiCQL/maGtBfDHNMDvyWgC9Ww//8pmlsU3RUs=
```

Setiap module biasanya punya dua baris:

| Baris | Isi yang di-hash |
|---|---|
| `<module> <versi> h1:...` | Seluruh isi source code module (file zip). |
| `<module> <versi>/go.mod h1:...` | Hanya file `go.mod` milik module tersebut. Dipakai untuk menghitung graph dependency tanpa mengunduh source lengkap. |

`h1:` menandakan algoritma hash SHA-256 yang di-encode base64.

### Kenapa `go.sum` penting

1. **Integritas.** Setiap kali module diunduh, Go mencocokkan hash-nya dengan `go.sum`. Jika berbeda, build gagal dengan error `checksum mismatch`. Ini melindungi proyek dari kode yang diubah diam-diam.
2. **Verifikasi global.** Saat module pertama kali ditambahkan, hash-nya juga dicocokkan dengan database checksum publik di `sum.golang.org`.
3. **Reproducible.** Semua developer dan server CI dijamin memakai source code yang persis sama.

> **`go.mod` dan `go.sum` wajib di-commit ke Git.** `go.sum` juga bisa memuat baris dari module yang tidak lagi dipakai langsung. Itu normal. Jalankan `go mod tidy` untuk merapikannya.

---

## Dependency langsung

Package berikut di-import langsung oleh kode proyek, dikelompokkan menurut
fungsinya. Versi lengkap ada di `go.mod`.

### HTTP dan gRPC

| Package | Fungsi | Dipakai di |
|---|---|---|
| [`github.com/gofiber/fiber/v3`](https://github.com/gofiber/fiber) | Web framework HTTP berbasis fasthttp: routing, middleware, binding, error handler, graceful shutdown. | `internal/config/fiber.go`, `internal/delivery/http/` |
| [`github.com/gofiber/storage/memory/v2` dan `.../redis/v3`](https://github.com/gofiber/storage) | Storage penghitung rate limiter: memori lokal (cadangan) dan Redis (bersama antarinstance). | `internal/delivery/http/middleware/storage.go`, `internal/config/web.go` |
| [`google.golang.org/grpc`](https://grpc.io/docs/languages/go/), [`google.golang.org/protobuf`](https://pkg.go.dev/google.golang.org/protobuf) | Server gRPC dan kode hasil generate dari `.proto`. | `internal/delivery/grpc/` |
| [`google.golang.org/genproto/googleapis/rpc`](https://pkg.go.dev/google.golang.org/genproto/googleapis/rpc) | Detail error gRPC terstruktur (`BadRequest`, `ErrorInfo`). | `internal/delivery/grpc/errors.go` |
| [`github.com/grpc-ecosystem/go-grpc-middleware/v2`](https://github.com/grpc-ecosystem/go-grpc-middleware) | Interceptor gRPC (recovery, logging, metrik Prometheus). | `internal/delivery/grpc/server.go` |
| [`golang.org/x/time`](https://pkg.go.dev/golang.org/x/time/rate) | Rate limiter token bucket untuk gRPC. | `internal/delivery/grpc/limits.go` |

### Data dan konfigurasi

| Package | Fungsi | Dipakai di |
|---|---|---|
| [`gorm.io/gorm`](https://gorm.io), [`gorm.io/driver/mysql`](https://github.com/go-gorm/mysql) | ORM dan driver MySQL. | `internal/config/database.go`, `internal/repository/` |
| [`gorm.io/plugin/soft_delete`](https://github.com/go-gorm/soft_delete) | Soft delete dengan kolom `deleted_at` BIGINT (0 = aktif). | `internal/entity/` |
| [`github.com/golang-migrate/migrate/v4`](https://github.com/golang-migrate/migrate) | Menjalankan migrasi SQL di integration test. Di lingkungan lain migrasi dijalankan lewat container `migrate` (`make migrate-up`). | `test/testsupport/` |
| [`github.com/spf13/viper`](https://github.com/spf13/viper) | Membaca `config.json` yang bisa ditimpa env berawalan `APP_`, misalnya `APP_DATABASE_PASSWORD`. | `internal/config/config.go` |
| [`github.com/go-playground/validator/v10`](https://github.com/go-playground/validator) | Validasi struct lewat tag, misalnya `validate:"required,max=100"`. | `internal/validation/` |
| [`github.com/google/uuid`](https://github.com/google/uuid) | Membuat UUID untuk ID data dan event. | `internal/usecase/`, `internal/model/` |

### Keamanan

| Package | Fungsi | Dipakai di |
|---|---|---|
| [`golang.org/x/crypto`](https://pkg.go.dev/golang.org/x/crypto) | `bcrypt` untuk hash password. | `internal/security/password.go` |
| [`github.com/golang-jwt/jwt/v5`](https://github.com/golang-jwt/jwt) | Menerbitkan dan memverifikasi access token JWT (HS256). | `internal/security/jwt.go` |

### Event, layanan luar, dan pekerjaan terjadwal

| Package | Fungsi | Dipakai di |
|---|---|---|
| [`github.com/IBM/sarama`](https://github.com/IBM/sarama) | Client Kafka: producer untuk relay outbox dan consumer group di worker. | `internal/config/kafka.go`, `internal/outbox/`, `internal/delivery/messaging/` |
| [`github.com/redis/go-redis/v9`](https://github.com/redis/go-redis) | Client Redis untuk cache, rate limit, dan lock scheduler. | `internal/config/redis.go`, `internal/gateway/cache/`, `internal/scheduler/` |
| [`github.com/minio/minio-go/v7`](https://github.com/minio/minio-go) | Client S3-compatible untuk upload avatar dan presigned URL. | `internal/gateway/storage/` |
| [`github.com/wneessen/go-mail`](https://github.com/wneessen/go-mail) | Mengirim email lewat SMTP. | `internal/gateway/email/` |
| [`github.com/go-co-op/gocron/v2`](https://github.com/go-co-op/gocron) | Scheduler cron untuk job pembersihan. | `internal/scheduler/` |

### Observability

| Package | Fungsi | Dipakai di |
|---|---|---|
| [`github.com/prometheus/client_golang`](https://github.com/prometheus/client_golang) | Metrik Prometheus dan endpoint `/metrics`. | `internal/observability/metrics.go` |
| [`go.opentelemetry.io/otel`](https://opentelemetry.io/docs/languages/go/) (+ `sdk`, `trace`, exporter `otlptracegrpc`) | Distributed tracing dan pengiriman span ke collector OTLP. | `internal/observability/tracing.go`, `internal/config/otel.go` |
| `gofiber/contrib/v3/otel`, `gorm.io/plugin/opentelemetry`, `redisotel`, `otelgrpc` | Instrumentasi tracing otomatis untuk Fiber, GORM, Redis, dan gRPC. | `internal/config/`, `internal/delivery/grpc/server.go` |

Untuk logging, proyek ini memakai [`log/slog`](https://pkg.go.dev/log/slog)
dari standard library, jadi tidak tercantum di `go.mod`. Logger dibuat di
`internal/config/logger.go`. Level dan format diatur lewat `log.level`
(`debug`, `info`, `warn`, `error`) dan `log.format` (`text` atau `json`)
di `config.json`, atau env `APP_LOG_LEVEL` dan `APP_LOG_FORMAT`.

### Khusus test

| Package | Fungsi | Dipakai di |
|---|---|---|
| [`github.com/stretchr/testify`](https://github.com/stretchr/testify) | `assert`, `require`, dan `mock` (dipakai mock hasil mockery). | `internal/**/*_test.go`, `test/` |
| [`github.com/testcontainers/testcontainers-go`](https://golang.testcontainers.org/) (+ modul `mysql`, `kafka`, `redis`) | Menjalankan container sungguhan di integration test. | `test/testsupport/` |
| [`gopkg.in/yaml.v3`](https://pkg.go.dev/gopkg.in/yaml.v3) | Membaca `api/openapi.yaml` untuk memastikan semua route terdokumentasi. | `test/docs_test.go` |

Dependency yang hanya dipakai test tetap tercantum di `go.mod`, tetapi
tidak ikut masuk ke binary production: linker Go hanya menyertakan package
yang benar-benar di-import oleh `cmd/web`, `cmd/worker`, atau `cmd/cli`.

---

## Dependency tidak langsung (indirect)

Baris bertanda `// indirect` (lebih dari seratus di proyek ini) adalah
module yang dibutuhkan oleh dependency di atas, bukan oleh kode proyek.
Sebagian besar berasal dari testcontainers (client Docker), OpenTelemetry,
dan gRPC. Beberapa yang menarik untuk dikenali:

| Package | Dibawa oleh | Fungsi |
|---|---|---|
| `github.com/valyala/fasthttp` | Fiber | HTTP server berperforma tinggi yang menjadi fondasi Fiber. |
| `github.com/go-sql-driver/mysql` | Driver MySQL GORM | Driver MySQL standar untuk `database/sql`. |
| `github.com/klauspost/compress`, `github.com/pierrec/lz4/v4` | Fiber, Sarama | Kompresi respons HTTP dan pesan Kafka. |
| `github.com/jcmturner/gokrb5/v8` | Sarama | Autentikasi Kerberos untuk Kafka (tidak dipakai proyek, tetapi ikut terbawa). |
| `github.com/moby/moby/client` | testcontainers | Client Docker untuk menjalankan container test. |

Untuk mengetahui mengapa sebuah module ada, jalankan:

```bash
go mod why -m github.com/jcmturner/gokrb5/v8
```

---

## Perintah yang sering dipakai

| Perintah | Fungsi |
|---|---|
| `go mod tidy` | Menambah dependency yang kurang, menghapus yang tidak dipakai, dan merapikan `go.sum`. |
| `go get <module>@<versi>` | Menambah atau mengubah versi dependency, misalnya `go get github.com/gofiber/fiber/v3@v3.6.1`. |
| `go get -u ./...` | Update semua dependency ke versi minor atau patch terbaru. |
| `go list -m -u all` | Melihat dependency yang punya versi baru. |
| `go mod why -m <module>` | Menjelaskan kenapa sebuah module dibutuhkan, termasuk rantai import-nya. |
| `go mod graph` | Menampilkan seluruh graph dependency. |
| `go mod verify` | Memastikan module di cache lokal cocok dengan `go.sum`. |
| `go run golang.org/x/vuln/cmd/govulncheck@latest ./...` | Memindai kerentanan keamanan yang benar-benar terpanggil oleh kode. |

Alur update yang disarankan:

```bash
go get -u ./...
go mod tidy
go build ./... && go vet ./...
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
go test ./...
```
