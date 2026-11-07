# 08. Autentikasi dan Keamanan

## Password: bcrypt

```go
// internal/security/password.go
hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
ok := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
```

- bcrypt **sengaja lambat** dan memakai salt acak, sehingga hash yang bocor
  sulit ditebak balik.
- bcrypt hanya membaca 72 byte pertama, karena itu validasi
  `max=72` mencegah bagian password diabaikan diam-diam.
- **Jangan pernah** menyimpan atau mencatat password asli.

### Mencegah user enumeration

Login dengan email tidak terdaftar dan login dengan password salah:

- mengembalikan **pesan yang sama** ("invalid email or password"), dan
- membutuhkan **waktu yang sama**: untuk email yang tidak ada, `CompareDummy`
  tetap menjalankan bcrypt terhadap hash dummy.

Tanpa ini, penyerang bisa mengetahui email mana yang terdaftar dari perbedaan
pesan atau waktu respons.

## Model dua token

| | Access token | Refresh token |
|---|---|---|
| Bentuk | JWT (bertanda tangan) | String acak 256-bit |
| Umur | 15 menit | 7 hari |
| Dikirim | Setiap request (`Authorization: Bearer`) | Hanya ke `/auth/refresh` |
| Diverifikasi | Tanpa database (cek tanda tangan) | Di database (sebagai hash) |
| Bisa dicabut instan | Tidak (tunggu kedaluwarsa) | Ya |

Access token yang berumur pendek membatasi kerugian bila token dicuri.
Refresh token memberi pengalaman "tetap login" tanpa harus login ulang
setiap 15 menit.

### Verifikasi JWT yang benar

```go
// internal/security/jwt.go
jwt.ParseWithClaims(token, claims, keyFunc,
    jwt.WithValidMethods([]string{"HS256"}),   // tolak "alg: none" dan RS256 palsu
    jwt.WithIssuer(t.issuer),
    jwt.WithAudience(t.audience),
    jwt.WithExpirationRequired(),
)
```

Kesalahan umum yang dicegah di sini: menerima algoritma apa pun yang
disebut di header token. Test yang menjaganya: `TestParse_RejectsAlgNone`
dan `TestParse_RejectsRS256Header`.

### Refresh token: disimpan sebagai hash, dirotasi, dan dideteksi reuse

1. **Hanya hash yang disimpan** (`SHA-256`). Bila database bocor, token
   aslinya tetap tidak diketahui. SHA-256 cukup (tidak perlu bcrypt) karena
   token sudah acak penuh.
2. **Rotasi**: setiap refresh mencabut token lama dan menerbitkan token
   baru dalam **keluarga** (`family_id`) yang sama.
3. **Deteksi reuse**: bila token yang sudah dicabut dipakai lagi, berarti
   token itu dicuri, dan **seluruh keluarga dicabut**.

```go
// internal/usecase/auth_usecase.go (diringkas)
old, err := c.deps.Tokens.FindByHashForUpdate(ctx, hash)   // kunci baris
if old.RevokedAt != nil {
    reused = true
    return c.deps.Tokens.RevokeFamily(ctx, old.FamilyID)    // commit pencabutan
}
...
if reused {
    return nil, apperror.Unauthorized("invalid refresh token") // tolak SETELAH commit
}
```

Perhatikan trik di sini: bila error dikembalikan **di dalam** transaksi,
pencabutan keluarga ikut di-rollback. Karena itu fungsi mengembalikan `nil`
(transaksi commit), lalu menolak request setelahnya.

### Trade-off yang disengaja

Logout dan ganti password mencabut refresh token, tetapi access token yang
sudah terbit tetap berlaku sampai 15 menit. Ini harga verifikasi tanpa
database. Bila butuh pencabutan instan, tambahkan denylist `jti` di Redis.

## Otorisasi (RBAC)

- Role (`user`/`admin`) ikut di dalam JWT.
- `middleware.RequireRole("admin")` menjaga grup `/api/v1/admin`.
- **Kepemilikan** tetap dicek di setiap query (`user_id = ?`). Role tidak
  menggantikan pengecekan kepemilikan.
- Aturan bisnis seperti "admin tidak boleh mengubah role dirinya sendiri"
  ada di use case (`AdminUseCase.ChangeRole`).

## Rate limiting

| Limiter | Kunci | Batas | Melindungi dari |
|---|---|---|---|
| Global | IP | 100/menit | Banjir request |
| Login/registrasi | IP + email | 5/menit | Menebak password satu akun |
| Login/registrasi | IP | 20/menit | Mencoba banyak akun dari satu IP |
| Login/registrasi | Email (hanya kegagalan) | 10/menit | Tebakan terdistribusi ke satu akun |
| Refresh | IP | 60/menit | Penyalahgunaan endpoint refresh |

Detail penting:

- **IP asli di belakang proxy.** `X-Forwarded-For` hanya dipercaya bila
  request datang dari `trusted_proxies`. Tanpa itu, client bisa
  memalsukan IP (`TestRateLimit_IgnoresSpoofedXForwardedFor`).
- **Bila Redis mati**, limiter global beralih ke memori (fail-open,
  supaya API tetap hidup), sedangkan limiter login dan refresh menolak
  dengan 503 (fail-closed, supaya penyerang tidak bisa memanfaatkan
  gangguan Redis).

## Header, input, dan batas

- **helmet**: header seperti `X-Content-Type-Options: nosniff`.
- **CORS**: hanya origin di allowlist; `*` ditolak di production.
- **Body JSON** maksimal 1 MB, dan hanya `application/json` yang diterima.
- **Upload file**: tipe file ditentukan dari **isinya**
  (`http.DetectContentType`), bukan dari nama file. File HTML yang diberi
  nama `foto.png` ditolak.

## Jangan bocorkan data sensitif

Kebocoran sering terjadi bukan di API, tetapi di tempat lain. Pelajaran
dari repo ini (beberapa ditemukan oleh security review selama pengembangan):

| Tempat | Risiko | Pencegahan |
|---|---|---|
| Log | Token tercetak | Log debug token dihapus, field sensitif tidak pernah di-log |
| Audit trail | Hash password tersimpan | `alwaysExcluded` di `internal/usecase/audit.go` |
| Event | Payload memuat password | Payload memakai model respons |
| Tracing (span) | Query SQL beserta nilainya (termasuk hash bcrypt) | GORM `WithoutQueryVariables` |
| Tracing (span) | Perintah Redis berisi email | redisotel `WithDBStatement(false)` |
| Tracing (span) | `?email=...` di URL | `WithRedactedQueryParams` |
| Readiness | Pesan error berisi host database | Hanya status `"unavailable"` |
| Email | Nama yang diisi bebas bisa berisi teks phishing | Email tidak memuat nama |
| Image Docker | Secret dev ikut ter-bundle | Image default production, validasi menolak secret dev |

## Pemeriksaan otomatis

- `gosec` (lewat golangci-lint): pola kode yang tidak aman.
- `govulncheck`: kerentanan di dependency **yang benar-benar dipanggil**.
- Dependabot: pembaruan dependency mingguan.

## Latihan

1. Tambahkan denylist `jti` di Redis: saat logout, simpan `jti` access token
   sampai waktu kedaluwarsanya, lalu periksa di middleware. Apa biayanya per request?
2. Tulis test yang membuktikan admin tidak bisa membaca kontak milik user
   lain lewat `GET /api/v1/contacts/{id}`.
3. Telusuri: apa yang terjadi bila dua request refresh dengan token yang
   sama datang bersamaan dan `FOR UPDATE` dihapus? Jalankan
   `TestRefresh_ConcurrentSameToken` untuk membuktikannya.
