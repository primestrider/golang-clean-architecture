# 06. HTTP API

Repo ini memakai **Fiber v3** (dibangun di atas fasthttp). Konsepnya sama
untuk framework lain (net/http standar, Gin, Echo, Chi).

## Route dan grup

```go
// internal/delivery/http/route/route.go
func (c *RouteConfig) Setup() {
    api := c.App.Group("/api/v1")
    c.setupGuestRoutes(api)                          // tanpa login
    authed := api.Group("", c.AuthMiddleware)        // wajib login
    c.setupAuthRoutes(authed)
    c.setupAdminRoutes(authed.Group("/admin", middleware.RequireRole(entity.RoleAdmin)))
}
```

- **Versi di URL** (`/api/v1`) memungkinkan perubahan besar dirilis sebagai
  `/api/v2` tanpa memutus client lama.
- **Middleware per grup**: semua route di `authed` melewati `AuthMiddleware`.
- **Middleware per route**: login dan registrasi diberi rate limiter khusus:

```go
api.Post("/auth/login", c.AuthIPRateLimit, c.AuthIPEmailRateLimit, c.AuthEmailRateLimit,
    middleware.RenderErrors(), c.AuthController.Login)
```

## Controller: tipis dan membosankan

Controller hanya menerjemahkan antara HTTP dan use case:

```go
// internal/delivery/http/contact_controller.go
func (h *ContactController) Create(c fiber.Ctx) error {
    req := new(model.CreateContactRequest)
    if err := c.Bind().Body(req); err != nil {
        return fiber.ErrBadRequest                    // JSON rusak → 400
    }
    req.UserID = middleware.CurrentAuth(c).UserID     // dari token, bukan body
    res, err := h.contacts.Create(c.Context(), req)
    if err != nil {
        return err                                    // biar error handler yang urus
    }
    return c.JSON(model.WebResponse[*model.ContactResponse]{Data: res})
}
```

Aturan yang diikuti:

- Tidak ada aturan bisnis di controller.
- Tidak ada logging error di controller; error handler global yang mencatatnya.
- Identitas user **selalu** dari token, tidak pernah dari body atau query.

## Validasi

Aturan ditulis sebagai tag pada model request, lalu dijalankan di **use
case** (bukan di controller), supaya gRPC juga mendapat validasi yang sama:

```go
Password string `json:"password" validate:"required,min=8,max=72"`
```

`internal/validation/validation.go` mengubah hasil validator menjadi
`apperror.Validation(fields)` dengan nama field dari tag JSON
(`first_name`, bukan `FirstName`) dan pesan yang mudah dipahami.

## Error: RFC 9457 Problem Details

Semua error dikirim dalam satu format standar:

```json
{
  "type": "https://problems.example.com/validation-failed",
  "title": "Bad Request",
  "status": 400,
  "detail": "request validation failed",
  "instance": "/api/v1/contacts",
  "request_id": "6c1b...",
  "errors": [{ "field": "email", "message": "must be a valid email" }]
}
```

Alurnya:

1. Use case mengembalikan `apperror.Error` dengan `Code` netral.
2. `HTTPStatus(code)` memetakan `CodeNotFound` ke 404, `CodeConflict` ke 409,
   dan seterusnya.
3. Error tak terduga (misalnya database mati) menjadi 500 dengan pesan
   generik; detailnya hanya masuk log.
4. `request_id` memudahkan menghubungkan laporan pengguna dengan log server.

Kode lengkapnya ada di `internal/delivery/http/problem.go`.

## Middleware: urutan itu penting

`internal/delivery/http/middleware/stack.go` memasang middleware dengan
urutan yang disengaja:

| Urutan | Middleware | Alasan posisinya |
|---|---|---|
| 1 | recover | Paling luar, supaya panic di mana pun tertangkap |
| 2 | request id | Supaya semua log setelahnya punya id |
| 3 | tracing, metrics | Mengukur seluruh pemrosesan |
| 4 | access log | Mencatat status final (ia juga merender error) |
| 5 | helmet | Header keamanan browser |
| 6 | CORS | Menolak origin asing lebih awal |
| 7 | rate limit global | Melindungi server dari banjir request |
| 8 | timeout | Membatasi waktu kerja |
| 9 | wajib JSON | 415 untuk Content-Type lain, 413 untuk body terlalu besar |

Middleware di Fiber adalah handler yang memanggil `c.Next()`. Kode sebelum
`c.Next()` berjalan saat request masuk; kode sesudahnya berjalan saat
respons keluar. Lihat `accessLog` sebagai contoh.

## Respons sukses yang konsisten

```go
type WebResponse[T any] struct {
    Data   T             `json:"data"`
    Paging *PageMetadata `json:"paging,omitempty"`
}
```

Daftar kontak memakai paging `{page, size, total_item, total_page}`. Status
`201 Created` dipakai untuk registrasi, dan `204 No Content` untuk logout serta
hapus avatar.

## Dokumentasi API

- `api/openapi.yaml` ditulis tangan dan disajikan di `/docs` (Swagger UI).
- `TestOpenAPI_CoversAllRoutes` **gagal bila ada route yang belum
  terdokumentasi**, jadi dokumentasi tidak bisa tertinggal dari kode.

## Delivery kedua: gRPC

`internal/delivery/grpc` menunjukkan bahwa protokol hanyalah lapisan luar:

- Kontrak ada di `api/proto/contact/v1/contact.proto`, dan kode di-generate
  dengan `make proto`.
- `toStatus` memetakan `apperror.Code` ke `codes.Code` (misalnya
  `CodeConflict` ke `codes.Aborted`).
- Interceptor adalah padanan middleware: recovery, request id, logging,
  metrics, rate limit, timeout, dan auth.

## Latihan

1. Tambahkan query `sort=first_name|created_at` ke `GET /api/v1/contacts`.
   Di layer mana validasi nilai `sort` sebaiknya dilakukan? Jangan lupa
   memperbarui `openapi.yaml`.
2. Ubah respons timeout (saat ini 500) menjadi 504. Petunjuk:
   `context.DeadlineExceeded` dan `apperror.From`.
3. Coba kirim `POST /api/v1/contacts` dengan `Content-Type: text/plain` dan
   dengan JSON rusak. Jelaskan jalur kode yang menghasilkan 415 dan 400.
