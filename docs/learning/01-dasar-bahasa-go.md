# 01. Dasar Bahasa Go

Bab ini membahas fitur bahasa Go yang paling sering muncul di repo, masing-
masing dengan contoh nyata.

## Module dan package

- **Module** adalah satu proyek dengan satu file `go.mod`. Baris pertamanya
  adalah nama module:

  ```
  module github.com/primestrider/golang-clean-architecture
  ```

- **Package** adalah satu folder. Semua file di folder yang sama memakai
  baris `package` yang sama. Package diimpor dengan path lengkap:

  ```go
  import "github.com/primestrider/golang-clean-architecture/internal/apperror"
  ```

- **Huruf besar = diekspor.** `apperror.NotFound` bisa dipakai dari package
  lain. Fungsi berhuruf kecil seperti `toAppError` di
  `internal/delivery/http/problem.go` hanya bisa dipakai di package-nya sendiri.
- File `doc.go` di setiap package berisi penjelasan package itu. Coba baca
  `internal/usecase/doc.go`.

## Variabel, tipe, dan zero value

```go
var count int64          // nilai awal 0
name := "Budi"           // tipe disimpulkan: string
var user *entity.User    // nilai awal nil
```

Setiap tipe punya **zero value**: `0`, `""`, `false`, `nil`. Go tidak punya
"undefined". Repo ini memanfaatkannya di beberapa tempat:

- `deleted_at = 0` berarti data masih aktif (soft delete,
  `internal/entity/user_entity.go`).
- Field config yang tidak diisi bernilai nol, lalu dicek di `Config.Validate()`.

## Struct dan struct tag

Struct adalah kumpulan field. **Tag** (teks di antara backtick) memberi
metadata untuk library: nama kolom database, nama field JSON, dan aturan validasi.

```go
// internal/model/contact_model.go
type CreateContactRequest struct {
    UserID    string `json:"-" validate:"required"`
    FirstName string `json:"first_name" validate:"required,max=100"`
    Email     string `json:"email" validate:"omitempty,max=200,email"`
}
```

- `json:"-"` berarti field ini **tidak pernah dibaca dari body**. `UserID`
  diisi dari token login, jadi client tidak bisa memalsukannya. Ini contoh
  keamanan yang dicapai lewat desain tipe.
- `validate:"..."` dibaca oleh go-playground/validator
  (`internal/validation/validation.go`).
- `gorm:"column:..."` di `internal/entity` dibaca oleh GORM.
- `mapstructure:"..."` di `internal/config/config.go` dibaca oleh Viper.

## Fungsi dan banyak nilai kembalian

Fungsi Go bisa mengembalikan lebih dari satu nilai. Polanya hampir selalu
`(hasil, error)`:

```go
// internal/usecase/contact_usecase.go
func (c *ContactUseCase) Get(ctx context.Context, req *model.GetContactRequest) (*model.ContactResponse, error) {
    if err := c.validate.Struct(req); err != nil {
        return nil, err
    }
    contact, err := c.contacts.FindByIDAndUserID(ctx, req.ID, req.UserID)
    if err != nil {
        return nil, err
    }
    return converter.ContactToResponse(contact), nil
}
```

Bentuk `if err := ...; err != nil` membatasi umur variabel `err` hanya di
dalam blok `if`.

## Method dan receiver

Method adalah fungsi yang menempel pada sebuah tipe:

```go
// internal/entity/user_entity.go
func (u *User) TableName() string { return "users" }
```

- **Receiver pointer (`*User`)** dipakai bila method mengubah isi struct
  atau struct-nya besar. Hampir semua use case dan repository di repo ini
  memakai receiver pointer.
- **Receiver nilai (`User`)** menerima salinan. Contohnya `NoopCache` di
  `internal/gateway/cache/cache.go`, yang tidak punya state.

## Pointer

`&x` mengambil alamat, `*p` membaca isi alamat. Di repo ini pointer dipakai untuk:

1. **Membedakan "kosong" dan "tidak ada".** `AvatarKey *string` di entity
   user: `nil` berarti belum punya avatar, berbeda dari string kosong.
   Begitu juga `RevokedAt *int64` di refresh token: `nil` berarti belum dicabut.
2. **Berbagi data tanpa menyalin.** Use case menerima `*model.CreateContactRequest`.
3. **Mengubah nilai dari dalam fungsi.** `NormalizeEmail` dipakai seperti
   `req.Email = NormalizeEmail(req.Email)` sebelum validasi.

## Slice dan map

```go
// membuat slice dengan panjang tertentu lalu mengisinya
responses := make([]model.ContactResponse, len(contacts))
for i := range contacts {
    responses[i] = *converter.ContactToResponse(&contacts[i])
}
```

Perhatikan `&contacts[i]`, bukan variabel perulangan. Ini mengambil alamat
elemen slice yang asli.

Map dipakai sebagai lookup:

```go
// internal/usecase/avatar_usecase.go
var allowedAvatarTypes = map[string]string{
    "image/jpeg": "jpg",
    "image/png":  "png",
}
ext, ok := allowedAvatarTypes[contentType]   // ok = false bila key tidak ada
```

Pola `nilai, ok := m[key]` adalah cara Go membedakan "tidak ada" dari zero value.

## Error adalah nilai

Go tidak punya exception. Error adalah nilai biasa yang dikembalikan dan
diperiksa. Repo ini punya tipe error sendiri:

```go
// internal/apperror/apperror.go
type Error struct {
    Code    Code
    Message string        // aman untuk client
    Fields  []FieldError
    Err     error         // penyebab asli, hanya untuk log
}

func (e *Error) Error() string { ... }  // memenuhi interface error
func (e *Error) Unwrap() error { return e.Err }
```

Tiga alat penting:

| Alat | Kegunaan | Contoh di repo |
|---|---|---|
| `fmt.Errorf("...: %w", err)` | Membungkus error sambil menambah konteks | `internal/config/database.go` |
| `errors.Is(err, target)` | Apakah rantai error berisi error tertentu? | `errors.Is(err, gorm.ErrRecordNotFound)` di `repository.go` |
| `errors.As` / `errors.AsType` | Ambil error bertipe tertentu dari rantai | `apperror.As(err)` |

**Prinsip di repo ini:** pesan untuk client (`Message`) dipisah dari detail
teknis (`Err`). Error 500 selalu memakai pesan generik
(`apperror.Internal`), supaya DSN atau nama tabel tidak bocor ke client.

## `defer`

`defer` menjalankan sebuah pemanggilan saat fungsi selesai, apa pun cara
keluarnya. Cocok untuk membersihkan sumber daya:

```go
resp, err := app.Test(req)
require.NoError(t, err)
defer resp.Body.Close()
```

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
```

## Closure dan fungsi sebagai nilai

Fungsi bisa disimpan di variabel dan "menangkap" variabel di sekitarnya.
Hampir semua middleware di repo ini adalah closure:

```go
// internal/delivery/http/middleware/auth_middleware.go
func NewAuth(tokens TokenParser) fiber.Handler {
    return func(c fiber.Ctx) error {   // fungsi ini "mengingat" tokens
        ...
        claims, err := tokens.Parse(token)
        ...
        return c.Next()
    }
}
```

Pola **fungsi yang mengembalikan fungsi** ini membuat dependency (`tokens`)
disuntikkan sekali saat setup, lalu dipakai di setiap request.

## Konstanta dan `iota`

```go
// internal/observability/log.go
type ctxKey int

const (
    requestIDKey ctxKey = iota  // 0
    userIDKey                   // 1
)
```

Tipe kunci privat (`ctxKey`) mencegah tabrakan kunci context dengan package lain.

## Type assertion

Untuk mengambil nilai konkret dari `any` (interface kosong):

```go
auth, _ := c.Locals(authLocalsKey).(*model.Auth)   // aman: tidak panic bila salah tipe
```

Tanpa `, ok` (atau `, _`), type assertion yang salah akan panic.

## `go:embed`

File bisa dimasukkan ke dalam binary saat kompilasi:

```go
// api/embed.go
//go:embed openapi.yaml
var OpenAPI []byte
```

Repo ini memakainya untuk spesifikasi OpenAPI dan template email
(`internal/usecase/templates`), sehingga tidak ada file tambahan yang harus
disalin saat deploy.

## Latihan

1. Di `internal/model/contact_model.go`, apa yang terjadi bila tag
   `json:"-"` pada `UserID` dihapus? Mengapa itu berbahaya?
2. Ubah `apperror.NotFound` agar menghasilkan pesan `"<resource> tidak
   ditemukan"`, jalankan `go test ./internal/...`, lalu lihat test mana yang
   gagal dan mengapa. Setelah itu kembalikan.
3. Tulis fungsi `func Truncate(s string, max int) string` dengan test-nya.
   Bandingkan dengan `truncate` di `internal/usecase/auth_usecase.go`. Apa
   yang terjadi pada karakter multi-byte (misalnya emoji)?
