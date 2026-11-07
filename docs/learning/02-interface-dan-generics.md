# 02. Interface dan Generics

## Interface dipenuhi secara implisit

Di Go, sebuah tipe memenuhi interface **cukup dengan punya method-nya**.
Tidak ada kata `implements`.

```go
// internal/usecase/ports.go
type Cache interface {
    Get(ctx context.Context, key string, dst any) (bool, error)
    Set(ctx context.Context, key string, value any, ttl time.Duration) error
    Delete(ctx context.Context, keys ...string) error
}
```

Dua tipe di `internal/gateway/cache/cache.go` memenuhinya tanpa saling kenal:
`RedisCache` (menyimpan ke Redis) dan `NoopCache` (tidak menyimpan apa pun).
`internal/config/web.go` memilih salah satunya saat runtime:

```go
var contactCache usecase.Cache = cache.NoopCache{}
if deps.Redis != nil {
    contactCache = cache.NewRedisCache(deps.Redis)
}
```

`ContactUseCase` tidak tahu dan tidak peduli implementasi mana yang dipakai.
Ini inti **dependency inversion**.

## Interface didefinisikan di sisi pemakai

Perhatikan bahwa interface `Cache`, `ContactRepository`, `Mailer`, dan
lainnya didefinisikan di **package `usecase`** (pemakai), bukan di package
`repository` atau `gateway` (pembuat). Ini idiom Go yang penting:

> *"Accept interfaces, return structs."*

- Pemakai hanya mendeklarasikan method yang ia butuhkan, sehingga
  interface-nya kecil.
- Implementasi tidak perlu mengimpor package pemakai, sehingga tidak ada
  import cycle.
- Unit test cukup menyediakan mock dari interface kecil itu.

Bandingkan dua hal: `repository.ContactRepository` adalah **struct** dengan
banyak method, sedangkan `usecase.ContactRepository` adalah **interface**
yang hanya berisi yang dipakai use case.

## Interface kecil lebih baik

Contoh interface satu method di repo:

```go
// internal/outbox/relay.go
type Dispatcher interface {
    Dispatch(ctx context.Context, e entity.OutboxEvent) error
}
```

Karena kecil, mudah dibuat beberapa implementasi: `KafkaDispatcher`,
`LocalDispatcher` (`internal/delivery/messaging/registry.go`), dan dispatcher
palsu di test (`flakyDispatcher` di `test/relay_test.go`).

Interface standar Go yang paling terkenal, `io.Reader`, juga hanya satu
method. Repo ini memakainya di upload avatar (`ObjectStorage.Put` menerima
`io.Reader`).

## Interface "anonim" dan pengecekan kemampuan

Kadang kita hanya perlu tahu apakah sebuah nilai punya method tertentu:

```go
// internal/outbox/relay.go: IsTransient
var temp interface{ IsTemp() bool }
return errors.As(err, &temp) && temp.IsTemp()
```

Kode ini bekerja dengan error SMTP dari go-mail **tanpa mengimpor go-mail**.
Inilah kekuatan interface implisit.

## Embedding struct

Go tidak punya inheritance. Sebagai gantinya ada **embedding**: method milik
struct yang di-embed "naik" ke struct luar.

```go
// internal/repository/contact_repository.go
type ContactRepository struct {
    Repository[entity.Contact]   // embed: Create, Delete, FindByID, ... ikut tersedia
}
```

`ContactRepository` otomatis punya `Create`, `Delete`, dan `FindByID`, lalu
menambahkan method khusus seperti `FindByIDAndUserID` dan `Search`.

Embedding juga dipakai pada interface. `contactv1.UnimplementedContactServiceServer`
di-embed di `ContactService` (gRPC) supaya kode tetap terkompilasi bila RPC baru
ditambahkan ke proto.

## Generics

Sejak Go 1.18, fungsi dan tipe bisa memakai **parameter tipe**:

```go
// internal/repository/repository.go
type Repository[T any] struct {
    DB   *gorm.DB
    Name string
}

func (r *Repository[T]) FindByID(ctx context.Context, id string) (*T, error) {
    return r.take(r.db(ctx).Where("id = ?", id))
}
```

Satu implementasi dipakai untuk `entity.User`, `entity.Contact`,
`entity.Address`, dan entity lainnya. Tanpa generics, kode ini harus ditulis
ulang untuk setiap entity, atau memakai `any` dengan type assertion yang
rawan error.

Contoh generics lain:

- `model.WebResponse[T any]` adalah amplop respons `{"data": T}` untuk tipe apa pun.
- `testsupport.DecodeData[T any](t, raw)` men-decode field `data` ke tipe tertentu.
- `errors.AsType[*apperror.Error](err)` dari library standar.

### Generics bertemu interface: optimistic locking

`UpdateWithVersion` harus membaca dan mengubah `Version` milik entity apa
pun. Caranya, ia memeriksa apakah `*T` memenuhi interface kecil:

```go
type versioned interface {
    GetID() string
    GetVersion() int
    SetVersion(v int)
}

func (r *Repository[T]) UpdateWithVersion(ctx context.Context, entity *T, expectedVersion int) error {
    v, ok := any(entity).(versioned)
    if !ok { ... }
    ...
}
```

## Kapan sebaiknya *tidak* membuat interface

Repo ini sengaja **tidak** membuat interface untuk semua hal:

- Controller memegang `*usecase.ContactUseCase` (struct konkret), karena
  controller tidak di-unit-test terpisah dan tidak punya implementasi kedua.
- `validation.Validator` dipakai langsung karena perilakunya deterministik
  dan cepat; tidak perlu di-mock.

Aturan praktis: buat interface ketika ada **lebih dari satu implementasi**
(nyata, atau mock di test) atau ketika perlu **memutus arah dependency**.

## Latihan

1. Buat implementasi `usecase.Cache` baru bernama `MemoryCache` (map plus
   `sync.Mutex`, dengan TTL) di `internal/gateway/cache`. Pasang di
   `web.go` saat Redis mati. Apa kelemahannya bila aplikasi berjalan di tiga
   instance?
2. Mengapa `usecase.TokenIssuer` hanya punya method `Issue`, sedangkan
   middleware memakai interface `TokenParser` dengan method `Parse`? Apa
   manfaatnya memisah keduanya?
3. Tambahkan method generik `Exists(ctx, id string) (bool, error)` ke
   `Repository[T]` beserta integration test-nya.
