# 09. Event-Driven Architecture

## Mengapa event?

Setelah user mendaftar, kita ingin mengirim email selamat datang. Bila email
dikirim langsung di request registrasi:

- registrasi menjadi lambat (menunggu server SMTP), dan
- registrasi gagal bila server email sedang down.

Lebih baik use case cukup **mengumumkan** "user telah terdaftar" (sebuah
*event*), lalu pihak lain bereaksi secara asinkron.

## Envelope event

```go
// internal/model/event.go
type Event struct {
    EventID     string          `json:"event_id"`     // unik, untuk idempotency
    EventType   string          `json:"event_type"`   // "contact.created"
    OccurredAt  time.Time       `json:"occurred_at"`
    AggregateID string          `json:"aggregate_id"` // id entity, juga key Kafka
    ActorID     string          `json:"actor_id,omitempty"`
    Data        json.RawMessage `json:"data"`
}
```

`AggregateID` menjadi **key pesan Kafka**, sehingga semua event milik entity
yang sama masuk ke partisi yang sama dan dibaca sesuai urutan.

## Masalah "dual write"

Cara naif:

```go
db.Save(contact)        // 1. simpan ke database
kafka.Send(event)       // 2. kirim ke Kafka
```

- Bila langkah 2 gagal, data ada tetapi event hilang.
- Bila langkah 1 di-rollback setelah langkah 2 berhasil, consumer menerima
  event untuk data yang tidak pernah ada.

Tidak ada transaksi yang mencakup database **dan** Kafka sekaligus.

## Solusi: transactional outbox

```go
// internal/usecase/contact_usecase.go
c.tx.WithinTransaction(ctx, func(ctx context.Context) error {
    c.contacts.Create(ctx, contact)                               // tabel contacts
    return publish(ctx, c.events, "contact.created", contact.ID, ...) // tabel outbox_events
})
```

1. Event ditulis ke tabel `outbox_events` **di transaksi yang sama**. Data
   dan event selalu konsisten: keduanya tersimpan, atau tidak sama sekali.
2. **Relay** di worker (`internal/outbox/relay.go`) membaca event pending
   setiap detik dan mengirimnya.
3. Setelah terkirim, event ditandai `published`.

## Relay: satu leader, urutan terjaga

- **Leader election lewat MySQL `GET_LOCK`** (`internal/outbox/locker.go`):
  dari banyak replica worker, hanya satu yang menjalankan relay. Bila worker
  itu mati, koneksinya putus, MySQL melepas lock, dan replica lain mengambil alih.
- **Urutan per aggregate**: bila event A1 gagal dikirim, event A2 (aggregate
  yang sama) tidak dikirim di batch itu. Aggregate lain tetap jalan.
- **Membedakan gangguan dan kesalahan**:
  - Broker mati atau jaringan putus (`outbox.IsTransient`): batch berhenti
    dan event tetap pending, tanpa dihitung sebagai percobaan. Event
    terkirim otomatis begitu broker hidup lagi.
  - Error permanen: dihitung, dan setelah `max_attempts` event **diparkir**
    (`failed`). Event yang diparkir terlihat di metrik `outbox_failed` dan
    perlu ditangani operator.

Pelajaran nyata: versi awal menghitung gangguan broker sebagai percobaan
gagal, sehingga Kafka yang mati 10 detik membuat semua event diparkir. Test
awalnya memakai `MaxAttempts: 1000` sehingga bug ini tertutup. Reviewer
menemukannya, dan test-nya sekarang memakai nilai default.

## At-least-once dan idempotency

Relay mengirim event lalu menandai `published`. Bila worker mati **di antara**
keduanya, event akan dikirim ulang. Jaminannya *at-least-once*: event tidak
pernah hilang, tetapi bisa datang lebih dari sekali.

Karena itu **consumer harus idempoten**:

```go
// internal/delivery/messaging/registry.go
r.tx.WithinTransaction(ctx, func(ctx context.Context) error {
    fresh, err := r.processed.TryMark(ctx, reg.consumer, e.EventID)  // INSERT IGNORE
    if !fresh {
        return nil              // sudah pernah diproses → lewati
    }
    return reg.handler(ctx, e)
})
```

Tabel `processed_events` memakai primary key `(consumer_name, event_id)`,
sehingga event yang sama tidak bisa diproses dua kali oleh consumer yang sama.
`TestWelcomeEmail_SentOnceViaLocalDispatcher` membuktikan email tidak
terkirim dua kali.

## Retry dan dead-letter queue (DLQ)

Di consumer Kafka (`internal/delivery/messaging/processor.go`):

1. Handler dicoba hingga 1 + 3 kali dengan jeda yang naik dua kali lipat
   (*exponential backoff*).
2. Bila tetap gagal, pesan dipindah ke topik `<topik>.dlq` dengan header
   alasan kegagalannya, lalu offset di-commit.
3. Pesan rusak (JSON tidak valid) langsung masuk DLQ.

Tanpa DLQ, satu pesan rusak bisa menghentikan seluruh partisi selamanya.

## Mode tanpa Kafka

Bila `kafka.enabled=false`, relay memakai `LocalDispatcher`: event langsung
diberikan ke registry handler di proses worker. Kode handler-nya sama, jadi
email selamat datang tetap terkirim di mode minimal.

## Menambah reaksi terhadap event

```go
// internal/config/worker.go: registerHandlers
registry.Register("welcome-email", "user.registered", notifications.SendWelcome)
```

Nama consumer (`"welcome-email"`) adalah kunci idempotency. **Jangan
diganti** setelah dipakai di production, karena semua event lama akan
dianggap belum diproses.

## Latihan

1. Tambahkan handler `"contact-counter"` untuk `contact.created` yang
   menaikkan kolom penghitung kontak di tabel users. Pastikan idempoten.
2. Apa yang terjadi bila handler mengirim email lalu commit transaksi
   `processed_events` gagal? Apakah email bisa terkirim dua kali? Bagaimana
   cara menguranginya?
3. Buka metrik `outbox_pending` di `/metrics` (port 9090/9091) saat Kafka
   dimatikan dan dinyalakan lagi di mode lengkap docker-compose.
