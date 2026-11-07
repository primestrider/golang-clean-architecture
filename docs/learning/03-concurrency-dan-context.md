# 03. Concurrency dan Context

Go dirancang untuk program yang mengerjakan banyak hal sekaligus. Server
HTTP Go menjalankan setiap request di goroutine tersendiri, jadi backend
Go selalu concurrent, sadar atau tidak.

## Goroutine

Goroutine adalah fungsi yang berjalan bersamaan, dimulai dengan kata `go`.
Biayanya sangat murah, jadi ribuan goroutine itu hal biasa.

```go
// cmd/web/main.go
serverErr := make(chan error, 3)
go func() { serverErr <- adminServer.Start() }()
go func() { serverErr <- app.Listen(addr, fiber.ListenConfig{...}) }()
```

Server admin dan server HTTP berjalan di goroutine masing-masing, sementara
goroutine utama menunggu sinyal shutdown.

## Channel dan `select`

Channel adalah pipa antar goroutine. `select` menunggu beberapa channel
sekaligus dan menjalankan cabang yang siap lebih dulu:

```go
select {
case <-ctx.Done():                 // sinyal SIGINT/SIGTERM
    log.Info("shutdown signal received")
case runErr = <-serverErr:         // salah satu server berhenti sendiri
    log.Error("server stopped unexpectedly", "error", runErr)
}
```

Contoh lain ada di consumer Kafka (`internal/delivery/messaging/consumer.go`):
`select` antara pesan baru dan context yang dibatalkan.

Channel juga bisa menjadi sinyal "selesai" dengan menutupnya:

```go
// internal/delivery/grpc/server.go: Shutdown
done := make(chan struct{})
go func() {
    server.GracefulStop()
    close(done)
}()
select {
case <-done:                // berhenti dengan rapi
case <-ctx.Done():          // batas waktu habis
    server.Stop()           // putus paksa
    <-done
}
```

Ini **pola timeout** yang sangat umum: jalankan pekerjaan di goroutine, lalu
tunggu hasilnya atau batas waktunya.

## `sync.WaitGroup`: menunggu banyak goroutine

```go
// internal/config/worker.go
var wg sync.WaitGroup
for _, j := range w.jobs {
    wg.Go(func() {          // Go 1.25+: Add(1) + go + Done() sekaligus
        if err := j.run(ctx); err != nil { ... }
    })
}
wg.Wait()                    // tunggu semua job selesai
```

Saat shutdown, worker membatalkan `ctx`, lalu `wg.Wait()` memastikan semua
pekerjaan (relay, consumer, scheduler) benar-benar berhenti sebelum koneksi
database ditutup.

## Mutex: melindungi data bersama

Bila beberapa goroutine menulis data yang sama, lindungi dengan
`sync.Mutex`:

```go
// internal/outbox/locker.go
type MySQLLocker struct {
    mu   sync.Mutex
    conn *sql.Conn
}

func (l *MySQLLocker) TryLock(ctx context.Context) (bool, error) {
    l.mu.Lock()
    defer l.mu.Unlock()
    ...
}
```

Untuk satu angka sederhana, `sync/atomic` lebih ringan. Contohnya
`atomic.Bool` untuk status readiness di `internal/delivery/admin/server.go`.

**Race condition** (dua goroutine mengakses data yang sama tanpa
sinkronisasi) dideteksi dengan `go test -race`. CI repo ini menjalankannya.

## `context.Context`

`context.Context` membawa tiga hal melewati batas fungsi:

1. **Pembatalan.** Bila client memutus koneksi atau aplikasi shutdown,
   pekerjaan turunan ikut berhenti.
2. **Batas waktu.** Misalnya timeout per request.
3. **Nilai yang terikat request.** Misalnya request id, user id, transaksi
   database, dan trace.

Aturan idiomatik yang diikuti repo ini:

- `ctx` selalu menjadi **parameter pertama**: `func (c *ContactUseCase) Get(ctx context.Context, ...)`.
- Jangan simpan `ctx` di field struct; teruskan lewat parameter.
- Selalu panggil `cancel()` dari `WithTimeout`/`WithCancel` (biasanya
  dengan `defer`).

### Timeout per request

```go
// internal/delivery/http/middleware/stack.go
func timeout(d time.Duration) fiber.Handler {
    return func(c fiber.Ctx) error {
        ctx, cancel := context.WithTimeout(c.Context(), d)
        defer cancel()
        c.SetContext(ctx)
        return c.Next()
    }
}
```

Karena repository memakai `db.WithContext(ctx)`, query yang melewati batas
waktu dibatalkan oleh driver MySQL.

### Membawa nilai: transaksi lewat context

Bagian paling menarik di repo ini adalah transaksi database yang dibawa di context:

```go
// internal/repository/transactor.go
func (t *GormTransactor) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
    return t.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
        return fn(context.WithValue(ctx, txKey{}, tx))
    })
}

func DB(ctx context.Context, base *gorm.DB) *gorm.DB {
    if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
        return tx
    }
    return base.WithContext(ctx)
}
```

Use case cukup menulis `tx.WithinTransaction(ctx, func(ctx) error {...})`,
dan semua repository yang dipanggil dengan `ctx` itu otomatis memakai
transaksi yang sama. Detailnya dibahas di [bab 07](07-database.md).

### Membawa nilai: log yang sadar request

`observability.WithRequestID(ctx, id)` menyimpan id di context.
`ContextHandler` (`internal/observability/log.go`) lalu menambahkannya ke
setiap baris log yang memakai `log.InfoContext(ctx, ...)`.

### `context.WithoutCancel`

Kadang pekerjaan tetap harus berjalan walau context induknya sudah batal.
Contohnya melepas lock saat shutdown:

```go
// internal/outbox/relay.go
defer func() {
    _ = r.locker.Unlock(context.WithoutCancel(ctx))
}()
```

## Graceful shutdown

Gabungan semua konsep di atas terlihat di `cmd/web/main.go`:

1. `signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)` menghasilkan
   context yang batal saat sinyal datang.
2. Server dijalankan di goroutine.
3. `select` menunggu sinyal atau error.
4. Readiness diubah ke false supaya load balancer berhenti mengirim traffic.
5. Server dihentikan dengan batas waktu (`context.WithTimeout`).
6. Dependency ditutup **setelah** request yang sedang berjalan selesai.

## Kesalahan umum

| Kesalahan | Akibat | Cara di repo ini |
|---|---|---|
| Goroutine tanpa jalan keluar | Goroutine bocor | Semua loop memeriksa `ctx.Done()` |
| Lupa `cancel()` | Timer bocor | `defer cancel()` |
| Menunggu tanpa batas waktu | Shutdown macet | `select` dengan `shutdownCtx.Done()` |
| Map ditulis dari banyak goroutine | Panic "concurrent map writes" | `sync.Mutex` di limiter gRPC |
| Variabel perulangan ditangkap closure | Nilai salah (sebelum Go 1.22) | Go ≥1.22 membuat variabel per iterasi |

## Latihan

1. Di `internal/outbox/relay.go`, mengapa `ProcessBatch` memeriksa
   `ctx.Err()` di setiap event? Apa yang terjadi saat shutdown tanpa pengecekan itu?
2. Tulis fungsi `func Retry(ctx context.Context, attempts int, backoff time.Duration, fn func() error) error`
   yang berhenti bila `ctx` batal. Bandingkan dengan loop retry di
   `internal/delivery/messaging/processor.go`.
3. Jalankan `go test -race ./internal/...` di Linux, WSL, atau CI. Coba
   sengaja buat race (hapus mutex di limiter gRPC) dan lihat laporannya.
