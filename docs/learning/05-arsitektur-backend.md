# 05. Arsitektur Backend

## Masalah yang ingin dipecahkan

Backend yang tumbuh tanpa struktur biasanya berakhir dengan handler HTTP
yang berisi query SQL, aturan bisnis, panggilan ke Kafka, dan format JSON
sekaligus. Akibatnya kode sulit diuji, sulit diubah, dan logika bisnis
terikat pada satu framework.

**Clean architecture** memisahkan kode berdasarkan *alasan berubah*:

| Layer | Berubah bila... | Folder |
|---|---|---|
| Delivery | Protokol berubah (REST → gRPC, Fiber → net/http) | `internal/delivery` |
| Use case | Aturan bisnis berubah | `internal/usecase` |
| Repository / Gateway | Teknologi penyimpanan atau sistem luar berubah | `internal/repository`, `internal/gateway` |
| Entity / Model | Bentuk data berubah | `internal/entity`, `internal/model` |

## Aturan dependency

```
delivery ──▶ usecase ──▶ port (interface) ◀── repository / gateway
```

- **Use case tidak mengimpor Fiber, GORM, Kafka, atau Redis.** Cek dengan
  `grep -r gofiber internal/usecase`; hasilnya kosong.
- Repository dan gateway **mengimplementasikan** interface yang
  didefinisikan use case.
- Hanya `internal/config` yang mengenal semua layer, karena di sanalah
  semuanya dirangkai.

Bukti nyata manfaatnya: `ContactUseCase` dipakai oleh **dua delivery
sekaligus**, REST (`contact_controller.go`) dan gRPC (`contact_service.go`),
tanpa satu baris pun perubahan di use case.

## Entity vs model

- **Entity** (`internal/entity`) adalah bentuk data di **database**.
  `entity.User` punya `Password` (hash) dan `DeletedAt`.
- **Model** (`internal/model`) adalah bentuk data di **API dan event**.
  `model.UserResponse` tidak punya password sama sekali.
- **Converter** (`internal/model/converter`) adalah satu-satunya tempat
  konversi entity ke model, sehingga field sensitif tidak bocor tanpa sengaja.

Dengan pemisahan ini, kolom database bisa diganti tanpa mengubah kontrak
API, dan sebaliknya.

## Alur satu request: `POST /api/v1/contacts`

```
HTTP request
  │
  ▼ middleware (recover, request id, tracing, metrics, log, helmet, CORS, rate limit, timeout, auth)
  ▼ ContactController.Create        parse body, isi UserID dari token
  ▼ ContactUseCase.Create           validasi → transaksi {
                                        contacts.Create
                                        audit.Record("contact.created")
                                        publish(event "contact.created") → outbox
                                    }
  ▼ ContactResponse → WebResponse{data: ...} → JSON 200
```

Bila terjadi error di layer mana pun, fungsi cukup mengembalikan `error`.
Error handler global (`internal/delivery/http/problem.go`) mengubahnya
menjadi Problem Details dengan status yang tepat.

## Composition root: tanpa DI container

`internal/config/web.go` merangkai semuanya **secara manual**:

```go
func BootstrapWeb(deps *WebDeps) *WebComponents {
    tx := repository.NewTransactor(deps.DB)
    contactRepository := repository.NewContactRepository(deps.DB)
    ...
    contactUseCase := usecase.NewContactUseCase(deps.Log, validate, tx,
        contactRepository, addressRepository, events, audit, contactCache)
    ...
    routes := route.RouteConfig{ ContactController: http.NewContactController(contactUseCase), ... }
    routes.Setup()
}
```

Mengapa tidak memakai DI container (wire, fx)?

- Semua dependency terlihat jelas di satu file. Tidak ada sihir.
- Error wiring ditangkap compiler, bukan saat runtime.
- Lebih mudah dipahami saat belajar.

Biayanya, file wiring menjadi panjang. Untuk proyek sangat besar, Google
Wire (code generation) bisa dipertimbangkan.

## Config: dua belas faktor

Prinsip [12-factor app](https://12factor.net/config): **config di environment**.

- `config.json` berisi default non-rahasia.
- Setiap key bisa ditimpa env `APP_<SECTION>_<KEY>`.
- `Config.Validate()` menolak start bila konfigurasi production tidak aman.
  Gagal cepat lebih baik daripada berjalan diam-diam dengan secret dev.

Lihat `internal/config/config.go`.

## Fitur opsional dengan implementasi pengganti

Template ini harus bisa berjalan hanya dengan MySQL. Polanya:
**satu interface, dua implementasi**, dan pemilihannya di composition root.

| Port | Implementasi aktif | Pengganti saat mati |
|---|---|---|
| `Cache` | `RedisCache` | `NoopCache` |
| `Mailer` | `SMTPMailer` | `LogMailer` |
| `outbox.Dispatcher` | `KafkaDispatcher` | `LocalDispatcher` |
| `fiber.Storage` (rate limit) | Redis | memori |

Use case tidak pernah memeriksa `if redisEnabled`. Keputusan itu ada di satu
tempat saja.

## Dua binary dari satu kode

- `cmd/web` melayani request: cepat, stateless, bisa di-scale horizontal.
- `cmd/worker` menjalankan pekerjaan latar belakang: relay outbox, consumer,
  dan scheduler.

Keduanya berbagi package `internal/...`, tetapi bisa di-deploy dan di-scale
terpisah. Email yang lambat tidak memperlambat API.

## Latihan

1. Gambar ulang alur `POST /api/v1/auth/refresh` seperti contoh di atas,
   termasuk bagian yang memakai `SELECT ... FOR UPDATE`.
2. Bayangkan Anda ingin mengganti GORM dengan `database/sql` biasa. Layer
   mana yang berubah? Layer mana yang sama sekali tidak tersentuh?
3. Mengapa `UserResponse` dan `UserEvent` (dulu) dipisah dari
   `entity.User`? Berikan dua contoh masalah bila entity langsung dikirim
   sebagai JSON.
