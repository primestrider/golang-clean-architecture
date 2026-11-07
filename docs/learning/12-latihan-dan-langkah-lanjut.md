# 12. Latihan dan Langkah Lanjut

## Latihan bertingkat

### Tingkat 1: memahami kode

1. Telusuri `POST /api/v1/auth/login` dari route sampai repository, lalu
   tuliskan setiap file dan fungsi yang dilewati.
2. Jalankan `go doc -all ./internal/apperror` dan jelaskan mengapa
   `Internal()` selalu memakai pesan generik.
3. Baca `internal/repository/transactor.go`. Jelaskan dengan kata-kata
   sendiri bagaimana satu transaksi bisa dipakai oleh beberapa repository.
4. Jalankan seluruh unit test, lalu satu integration test dengan
   `GCA_TEST_LOG=1` dan baca log-nya.

### Tingkat 2: perubahan kecil

5. Tambahkan field `company` ke kontak (migrasi, entity, model, converter,
   OpenAPI, dan test).
6. Ubah respons timeout dari 500 menjadi 504.
7. Tambahkan validasi config: `outbox.poll_interval` harus > 0.
8. Tambahkan query `sort` untuk daftar kontak dengan nilai yang dibatasi.

### Tingkat 3: fitur baru

9. Kerjakan [tutorial bab 11](11-tutorial-fitur-baru.md) sampai semua test hijau.
10. Reset password lewat email: token sekali pakai (simpan sebagai hash),
    kedaluwarsa 30 menit, dan email dikirim lewat event dan handler.
11. Verifikasi email: setelah terverifikasi, email selamat datang boleh
    memuat nama user. Mengapa sebelumnya tidak boleh? (Lihat bab 08.)
12. Denylist `jti` di Redis supaya logout langsung mematikan access token.

### Tingkat 4: kualitas dan operasional

13. Tutup salah satu "minor yang ditunda" di catatan review, misalnya
    urutan event dengan UUIDv7 atau backoff di loop consumer Kafka. Tulis
    test yang gagal lebih dulu.
14. Tambahkan manifest Kubernetes (Deployment, Service, probe liveness dan
    readiness, preStop hook).
15. Ukur performa `GET /api/v1/contacts` dengan `k6` atau `hey`, lalu lihat
    metrik p95 di Prometheus. Coba aktifkan Redis cache dan bandingkan.

## Ringkasan prinsip yang dipakai di repo ini

| Prinsip | Wujud di kode |
|---|---|
| Logika bisnis bebas framework | `internal/usecase` tidak mengimpor Fiber, GORM, atau Kafka |
| Interface di sisi pemakai | `internal/usecase/ports.go` |
| Gagal cepat | `Config.Validate()` menolak config production yang tidak aman |
| Satu sumber kebenaran untuk error | `apperror` lalu Problem Details atau status gRPC |
| Konsistensi data dan event | Transactional outbox |
| At-least-once + idempotency | `processed_events` |
| Aman secara default | Secret dari env, image production, data sensitif tidak pernah di-log |
| Diamati sejak awal | slog + Prometheus + OpenTelemetry |
| Test sebagai jaring pengaman | Unit (mock), integration (container sungguhan), mutation check |

## Bahan bacaan lanjutan

**Bahasa Go**
- [A Tour of Go](https://go.dev/tour/): pengantar interaktif resmi.
- [Effective Go](https://go.dev/doc/effective_go) dan
  [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments): gaya idiomatik.
- *Learning Go* (Jon Bodner) dan *100 Go Mistakes and How to Avoid Them*
  (Teiva Harsanyi).
- [Go Concurrency Patterns: Context](https://go.dev/blog/context).

**Backend dan arsitektur**
- *Designing Data-Intensive Applications* (Martin Kleppmann): transaksi,
  replikasi, dan event.
- [microservices.io: Transactional Outbox](https://microservices.io/patterns/data/transactional-outbox.html).
- [The Twelve-Factor App](https://12factor.net/).
- [RFC 9457: Problem Details for HTTP APIs](https://www.rfc-editor.org/rfc/rfc9457).

**Keamanan**
- [OWASP API Security Top 10](https://owasp.org/API-Security/).
- [OWASP Authentication Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html).
- [OAuth 2.0 Security Best Current Practice](https://datatracker.ietf.org/doc/html/rfc9700),
  termasuk refresh token rotation.

**Observability**
- [OpenTelemetry Go](https://opentelemetry.io/docs/languages/go/).
- [Prometheus: Naming and label best practices](https://prometheus.io/docs/practices/naming/).

## Penutup

Cara terbaik belajar dari repo ini adalah **mengubahnya**: tambahkan
fitur, rusak sesuatu dengan sengaja, lihat test mana yang menangkapnya, lalu
perbaiki. Setiap kali Anda bingung "mengapa kode ini ditulis begini", baca
doc comment-nya. Hampir semua keputusan desain dijelaskan di sana.
