# 13. Integrasi Layanan: Redis, Scheduler, Upload File, dan Email

Backend jarang berdiri sendiri. Bab ini membahas empat integrasi yang ada
di repo ini dan pola yang membuat semuanya **opsional**, **mudah diuji**,
dan **tidak menjatuhkan aplikasi** saat layanan luarnya bermasalah.

| Integrasi | Port (interface) | Implementasi | Pengganti saat dimatikan |
|---|---|---|---|
| Cache | `usecase.Cache` | `gateway/cache.RedisCache` | `cache.NoopCache` |
| Scheduler | `gocron.Locker` | `scheduler.RedisLocker` | tanpa lock (satu replica) |
| Object storage | `usecase.ObjectStorage` | `gateway/storage.MinioStorage` | route avatar tidak didaftarkan |
| Email | `usecase.Mailer` | `gateway/email.SMTPMailer` | `email.LogMailer` |

Semuanya dinyalakan lewat config (`redis.enabled`, `storage.enabled`,
`email.enabled`), sehingga `go run ./cmd/web` tetap jalan tanpa satu pun
layanan tambahan.

---

## Pola umum: port, adapter, dan pengganti

Semua integrasi mengikuti tiga langkah yang sama:

1. **Port** di `internal/usecase/ports.go`, kecil dan hanya berisi yang
   dibutuhkan use case:

   ```go
   type Mailer interface {
       Send(ctx context.Context, msg model.EmailMessage) error
   }
   ```

2. **Adapter** di `internal/gateway/...` yang membungkus library pihak
   ketiga (go-redis, minio-go, go-mail). Hanya adapter yang mengimpor
   library itu.

3. **Pemilihan di composition root** (`internal/config`):

   ```go
   // internal/config/email.go
   func NewMailer(cfg *Config, log *slog.Logger) (usecase.Mailer, error) {
       if !cfg.Email.Enabled {
           return email.NewLogMailer(log), nil
       }
       return email.NewSMTPMailer(email.Options{ /* ... */ })
   }
   ```

Hasilnya, use case **tidak pernah** menulis `if mailer != nil`. Ini disebut
*Null Object pattern*: implementasi "kosong" yang memenuhi interface
menggantikan pengecekan nil di mana-mana.

---

## Redis sebagai cache

### Cache-aside

`ContactUseCase.Get` (`internal/usecase/contact_usecase.go`) memakai pola
**cache-aside**: aplikasi sendiri yang membaca dan mengisi cache.

```go
key := contactCacheKey(req.UserID, req.ID)
cached := new(model.ContactResponse)
if hit, err := c.cache.Get(ctx, key, cached); err != nil {
    c.log.WarnContext(ctx, "cache get failed, falling back to database", "key", key, "error", err)
} else if hit {
    return cached, nil                         // 1. cache hit
}

contact, err := c.contacts.FindByIDAndUserID(ctx, req.ID, req.UserID) // 2. miss → DB
if err != nil {
    return nil, err
}
res := converter.ContactToResponse(contact)
if err := c.cache.Set(ctx, key, res, contactCacheTTL); err != nil { // 3. isi cache
    c.log.WarnContext(ctx, "cache set failed", "key", key, "error", err)
}
return res, nil
```

Tiga keputusan penting di potongan ini:

- **Cache bukan sumber kebenaran.** Error Redis hanya dicatat, lalu request
  jatuh ke database. Redis mati berarti aplikasi lebih lambat, bukan rusak.
- **User ID ada di key** (`contact:<userID>:<contactID>`). Tanpa itu, data
  satu user bisa terbaca oleh user lain bila suatu saat ada bug di
  pengecekan kepemilikan. Key cache adalah bagian dari keamanan.
- **Yang disimpan adalah model respons**, bukan entity. Field internal
  seperti `DeletedAt` tidak ikut ke cache.

### Invalidasi setelah commit

> "There are only two hard things in Computer Science: cache invalidation
> and naming things." (Phil Karlton)

Saat kontak diubah atau dihapus, cache dihapus **setelah transaksi
commit**:

```go
err := c.tx.WithinTransaction(ctx, func(ctx context.Context) error { /* ... */ })
if err != nil {
    return err
}
c.invalidate(ctx, req.UserID, req.ID)          // di LUAR transaksi
```

Mengapa tidak di dalam transaksi? Bayangkan urutan ini:

```
Request A (update)              Request B (get)
──────────────────              ───────────────
hapus cache
                                cache miss → baca DB (data LAMA, A belum commit)
                                isi cache dengan data LAMA
commit
```

Cache sekarang berisi data lama sampai TTL habis. Dengan menghapus setelah
commit, jendela kesalahan itu jauh lebih sempit. TTL 5 menit
(`contactCacheTTL`) adalah jaring pengaman terakhir bila penghapusan
gagal.

### Serialisasi dan tracing

`RedisCache` menyimpan nilai sebagai JSON (`encoding/json`). JSON mudah
dibaca saat debugging (`redis-cli GET contact:...`). Pilihan lain seperti
msgpack lebih ringkas tetapi lebih sulit diperiksa.

Di `internal/config/redis.go`, tracing Redis dipasang dengan
`redisotel.WithDBStatement(false)`. Tanpa opsi ini, isi perintah (termasuk
data kontak dan key rate limit yang memuat email) akan terkirim ke sistem
tracing. Ini contoh nyata bahwa **observability juga bisa membocorkan
data**.

### Redis untuk rate limiting

Redis juga menyimpan penghitung rate limit supaya batasnya berlaku lintas
instance. `internal/delivery/http/middleware/storage.go` punya dua
pembungkus dengan sikap berbeda saat Redis gagal:

| Limiter | Pembungkus | Saat Redis mati | Alasan |
|---|---|---|---|
| Global (beban) | `FailOpenStorage` | Pindah ke memori lokal | Lebih baik tetap melayani |
| Login/refresh (keamanan) | `FailClosedStorage` | Tolak dengan 503 | Penyerang tidak boleh memanfaatkan gangguan Redis untuk brute force |

Pelajarannya: **fail-open atau fail-closed adalah keputusan bisnis dan
keamanan**, bukan detail teknis.

---

## Scheduler: pekerjaan terjadwal

Worker menjalankan job pembersihan berkala dengan
[gocron v2](https://github.com/go-co-op/gocron) (`internal/scheduler`).

### Memisahkan "kapan" dari "apa"

```go
// internal/scheduler/jobs.go — APA yang dikerjakan
type Jobs struct {
    RefreshTokens RefreshTokenCleaner
    Outbox        OutboxCleaner
    Processed     ProcessedEventCleaner
    Now           func() time.Time
}

func (j Jobs) CleanupRefreshTokens(ctx context.Context) error {
    _, err := j.RefreshTokens.DeleteExpiredBefore(ctx, j.Now().Add(-refreshTokenRetention).UnixMilli())
    return err
}
```

```go
// internal/scheduler/scheduler.go — KAPAN dikerjakan
register("cleanup-refresh-tokens", opts.TokenCleanupCron, jobs.CleanupRefreshTokens)
```

Jadwalnya berasal dari config (`scheduler.token_cleanup_cron: "0 3 * * *"`,
yaitu setiap hari pukul 03.00 UTC). Karena `Jobs` berdiri sendiri dan
`Now` disuntikkan, `jobs_test.go` bisa menguji batas waktu (cutoff) tanpa
menunggu jam 3 pagi dan tanpa database.

### Masalah banyak replica

Bila worker dijalankan 3 replica, setiap jadwal akan dijalankan **tiga
kali**. Untuk job pembersihan, akibatnya hanya pemborosan. Untuk job
seperti "kirim tagihan bulanan", akibatnya fatal.

Solusinya adalah **distributed lock**. `RedisLocker` memakai `SET NX` dengan
masa berlaku:

```go
ok, err := l.client.SetNX(ctx, "scheduler:lock:"+key, "locked", l.ttl).Result()
if !ok {
    return nil, errors.New("lock held by another worker") // replica lain melewatkan jadwal ini
}
```

- `NX` berarti "set hanya bila belum ada". Operasi ini atomik di Redis,
  jadi hanya satu replica yang menang.
- **TTL wajib.** Bila replica pemegang lock mati sebelum `Unlock`, lock
  akan kedaluwarsa sendiri. TTL harus lebih lama dari durasi job.
- Tanpa Redis, scheduler berjalan tanpa lock dan worker mencatat log bahwa
  ia mengasumsikan hanya ada satu replica (`internal/config/worker.go`).

Bandingkan dengan relay outbox (bab 09) yang memakai `GET_LOCK` MySQL.
Relay berjalan **terus-menerus**, jadi butuh lock yang dipegang selama
koneksi hidup. Job terjadwal berjalan **sesekali**, jadi lock singkat
ber-TTL sudah cukup.

### Job harus idempotent

Lock bisa gagal di kasus tepi, misalnya job lebih lama dari TTL. Karena
itu job ditulis supaya **aman bila jalan dua kali**: "hapus yang lebih tua
dari X" menghasilkan keadaan akhir yang sama walau diulang.

---

## Upload file ke object storage (S3)

Foto profil disimpan di storage S3-compatible (AWS S3, MinIO, SeaweedFS,
Cloudflare R2). Kodenya ada di `internal/usecase/avatar_usecase.go` dan
`internal/gateway/storage/minio_storage.go`.

### Mengapa tidak disimpan di disk server?

- Container bersifat sementara; file di disk hilang saat redeploy.
- Beberapa replica tidak berbagi disk.
- Object storage murah, tahan lama, dan bisa diakses langsung lewat URL.

### Jangan percaya client

```go
const MaxAvatarSize = 2 << 20 // 2 MB

data, err := io.ReadAll(io.LimitReader(file, MaxAvatarSize+1))
if len(data) > MaxAvatarSize {
    return nil, apperror.New(apperror.CodePayloadTooLarge, "avatar must be at most 2 MB")
}
contentType := http.DetectContentType(data)
ext, ok := allowedAvatarTypes[contentType]   // hanya jpeg, png, webp
if !ok {
    return nil, apperror.New(apperror.CodeUnsupportedMedia, "avatar must be a JPEG, PNG, or WebP image")
}
```

- **Ukuran dibatasi dua kali.** Pertama oleh `BodyLimit` server
  (`internal/config/fiber.go`), lalu oleh `io.LimitReader`. Membaca
  `Max+1` byte adalah trik untuk mendeteksi file yang lebih besar dari klaim
  ukurannya tanpa membaca seluruh file.
- **Tipe file dari isi file** (`http.DetectContentType` membaca *magic
  bytes*), bukan dari nama file atau header `Content-Type`. Keduanya bisa
  dipalsukan: `virus.exe` bisa dikirim sebagai `foto.jpg`.
- **Nama object dibuat server**: `avatars/<userID>/<uuid>.<ext>`. Nama file
  dari user tidak pernah dipakai, sehingga tidak ada risiko path traversal
  (`../../etc/passwd`) atau tabrakan nama.

### Urutan operasi lintas sistem

Database dan S3 tidak bisa berada di satu transaksi. Urutannya dipilih
supaya kegagalan di titik mana pun tidak merusak data:

```
1. Upload object baru ke S3
2. Transaksi DB: simpan key baru di user
   ├─ gagal → hapus object baru (tidak ada yang menunjuk ke sana)
   └─ sukses → 3. hapus object lama
```

Bila langkah 3 gagal, yang tersisa hanya **object yatim**, yaitu file yang
tidak dipakai siapa pun. Itu pemborosan kecil, bukan data rusak. Yang
dihindari adalah kebalikannya: user yang menunjuk ke file yang sudah
hilang.

### Bucket privat + presigned URL

Bucket **tidak** dibuat publik. Saat respons user dibentuk, server membuat
URL bertanda tangan yang berlaku sementara (`storage.presign_ttl`, default
15 menit):

```go
url, err := c.storage.PresignGet(ctx, *user.AvatarKey, c.presignTTL)
```

URL ini bisa dibuka browser tanpa kredensial, tetapi kedaluwarsa sendiri.
`MinioStorage` punya dua client: satu untuk endpoint internal
(`s3:8333` di jaringan Docker) dan satu untuk menandatangani URL dengan
endpoint publik (`localhost:8333`). Tanda tangan S3 mencakup nama host,
jadi URL harus ditandatangani untuk host yang akan diakses client.

> **Alternatif untuk file besar:** *presigned PUT*. Server memberi URL
> upload, lalu client mengunggah **langsung ke S3** tanpa melewati server.
> Cara ini cocok untuk video atau dokumen besar. Repo ini memakai upload
> lewat server karena file kecil dan perlu divalidasi isinya.

---

## Email

### Dikirim dari worker, bukan dari request

Email selamat datang **tidak** dikirim di handler registrasi. Alurnya:

```
POST /auth/register ──▶ transaksi: simpan user + event user.registered (outbox)
                                     │
worker ◀── relay/Kafka ◀─────────────┘
   └─ handler "welcome-email" ──▶ NotificationUseCase.SendWelcome ──▶ SMTP
```

Pendaftarannya ada di `internal/config/worker.go`:

```go
registry.Register("welcome-email", "user.registered", notifications.SendWelcome)
```

Keuntungannya:

- Registrasi tetap cepat walau server SMTP lambat.
- Registrasi tidak gagal karena email gagal terkirim.
- Pengiriman yang gagal di-*retry* otomatis dan berakhir di DLQ bila terus
  gagal (bab 09).

Konsekuensinya, karena pengiriman bersifat **at-least-once**, email bisa
terkirim dua kali dalam kasus tepi. Tabel `processed_events` mencegah
sebagian besar duplikat.

### Template di dalam binary

```go
//go:embed templates/*
var templates embed.FS

var (
    welcomeHTML = htmltemplate.Must(htmltemplate.ParseFS(templates, "templates/welcome.html"))
    welcomeText = texttemplate.Must(texttemplate.ParseFS(templates, "templates/welcome.txt"))
)
```

- `go:embed` memasukkan file template ke dalam binary, jadi tidak ada file
  yang perlu disalin saat deploy.
- `template.Must` membuat aplikasi **gagal saat start** bila template
  rusak, bukan saat email pertama dikirim.
- Versi HTML memakai **`html/template`**, yang meng-escape data secara
  otomatis sesuai konteks (HTML, atribut, URL). Jangan pernah memakai
  `text/template` untuk HTML.
- Setiap email punya versi teks biasa untuk aplikasi email yang tidak
  menampilkan HTML.

### Keamanan konten email

`SendWelcome` sengaja **tidak** memuat nama yang diisi pendaftar. Alamat
email belum diverifikasi, jadi siapa pun bisa mendaftarkan email orang
lain dengan "nama" berisi teks phishing, misalnya *"Akun Anda diblokir,
klik link berikut"*. Email itu lalu terkirim dari domain resmi aplikasi.
Data dari user yang belum terverifikasi tidak boleh masuk ke email.

### Development tanpa server email sungguhan

- `email.enabled=false` memakai `LogMailer`, yang hanya mencatat subjek
  email (tanpa isi dan tanpa penerima, karena itu data pribadi).
- `make compose-full` (profil `full`) menjalankan **Mailpit** di port 1025
  (SMTP) dan <http://localhost:8025> (UI) untuk melihat email yang
  "terkirim".
- Di production, arahkan SMTP ke Amazon SES, SendGrid, Mailgun, atau
  sejenisnya, dengan `email.tls=true`.

---

## Menguji integrasi

| Jenis test | Cara | Contoh di repo |
|---|---|---|
| Unit (use case) | Mock port dengan mockery | `mocks.NewCache`, `mocks.NewObjectStorage`, `mocks.NewMailer` |
| Unit (logika murni) | Panggil fungsi langsung dengan `Now` tetap | `internal/scheduler/jobs_test.go` |
| Integration | Container sungguhan via testcontainers | Redis, SeaweedFS (S3), dan Mailpit di `test/testsupport` |

Yang penting diuji di integration test adalah hal-hal yang **tidak bisa**
dibuktikan mock, misalnya presigned URL benar-benar bisa diunduh, email
benar-benar sampai di Mailpit, dan cache benar-benar terhapus setelah
update.

---

## Latihan

1. Jalankan mode lengkap (`make compose-full`, yang juga mengaktifkan
   Redis), lalu panggil `GET /api/v1/contacts/{id}` dua kali. Periksa
   key-nya dengan `docker compose exec redis redis-cli KEYS 'contact:*'`
   dan `TTL`. Update kontaknya, lalu pastikan
   key itu hilang.
2. Matikan Redis saat aplikasi berjalan (`docker compose stop redis`).
   Apa yang terjadi pada `GET` kontak? Pada login? Cocokkan dengan tabel
   fail-open dan fail-closed di atas.
3. Tambahkan job terjadwal baru: hapus permanen kontak yang di-soft-delete
   lebih dari 90 hari lalu. Tulis test cutoff-nya lebih dulu di
   `jobs_test.go`.
4. Coba upload file teks yang diganti namanya menjadi `.png`. Status apa
   yang dikembalikan, dan baris kode mana yang menolaknya?
5. Tambahkan email "password Anda baru saja diubah" yang dikirim lewat
   event. Mengapa email jenis ini **boleh** memuat nama user?
6. **Desain:** cache juga daftar kontak (`Search`). Key apa yang Anda
   pakai, dan bagaimana menginvalidasinya saat satu kontak berubah?
   (Petunjuk: pertimbangkan nomor versi per user di key.)
