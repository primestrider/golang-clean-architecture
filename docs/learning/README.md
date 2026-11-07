# Belajar Go dan Backend dari Repo Ini

Seri materi ini memakai kode di repo ini sebagai bahan ajar. Setiap konsep
dijelaskan singkat, lalu ditunjukkan **di file mana konsep itu dipakai**,
supaya Anda bisa membuka kodenya, membaca doc comment-nya, dan
bereksperimen.

**Untuk siapa:** Anda sudah paham dasar pemrograman (variabel, fungsi,
perulangan) dan HTTP, tetapi baru mulai atau ingin mendalami Go untuk
backend.

**Cara belajar yang disarankan:**

1. Baca satu bab.
2. Buka file yang dirujuk dan baca kode serta komentarnya.
3. Kerjakan latihan di akhir bab.
4. Jalankan test (`go test ./internal/...`) untuk memastikan perubahan Anda
   tidak merusak apa pun.

## Jalur belajar

### Bagian 1: Bahasa Go lewat repo ini

| Bab | Isi |
|---|---|
| [00 Pendahuluan](00-pendahuluan.md) | Menyiapkan lingkungan, peta repo, cara membaca kode Go |
| [01 Dasar bahasa Go](01-dasar-bahasa-go.md) | Module, package, tipe, struct, method, pointer, slice, map, error, defer, closure |
| [02 Interface dan generics](02-interface-dan-generics.md) | Interface implisit, "accept interfaces", embedding, generics `Repository[T]` |
| [03 Concurrency dan context](03-concurrency-dan-context.md) | Goroutine, channel, `select`, `sync`, `context.Context`, graceful shutdown |
| [04 Testing di Go](04-testing-di-go.md) | `go test`, table test, testify, mock, integration test, mutation check |

### Bagian 2: Membangun backend dengan Go

| Bab | Isi |
|---|---|
| [05 Arsitektur backend](05-arsitektur-backend.md) | Clean architecture, layer, port, composition root, alur request |
| [06 HTTP API](06-http-api.md) | Fiber, route, controller, validasi, Problem Details, middleware |
| [07 Database](07-database.md) | GORM, repository, transaksi lewat context, migrasi, soft delete, optimistic locking |
| [08 Autentikasi dan keamanan](08-autentikasi-keamanan.md) | bcrypt, JWT, refresh token rotation, rate limit, RBAC, kebocoran data |
| [09 Event-driven](09-event-driven.md) | Transactional outbox, Kafka, idempotency, retry, DLQ |
| [10 Observability dan operasional](10-observability-operasional.md) | slog, Prometheus, OpenTelemetry, health check, config, Docker |

### Bagian 3: Praktik

| Bab | Isi |
|---|---|
| [11 Tutorial: membangun fitur "Catatan Kontak"](11-tutorial-fitur-baru.md) | Fitur lengkap dari migrasi sampai test, mengikuti pola repo |
| [12 Latihan dan langkah lanjut](12-latihan-dan-langkah-lanjut.md) | Daftar latihan bertingkat dan bahan bacaan lanjutan |

## Konvensi di materi ini

- `internal/usecase/contact_usecase.go:42` berarti file dan nomor baris
  (perkiraan; nomor baris bisa bergeser saat kode berubah).
- Potongan kode di materi ini **diringkas** untuk menonjolkan konsep. Versi
  lengkapnya ada di file yang dirujuk.
- Istilah teknis dibiarkan dalam bahasa Inggris bila itu istilah yang umum
  dipakai (misalnya *goroutine*, *middleware*, *transaction*).
