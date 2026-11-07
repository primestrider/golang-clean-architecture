# 14. Performa: Pagination, Benchmark, Profiling, dan Fuzzing

Aturan pertama optimasi: **ukur dulu**. Bab ini membahas cara Go membantu
Anda mengukur (benchmark, pprof), menguji input yang tidak terduga
(fuzzing), dan satu sumber masalah performa paling umum di backend, yaitu
daftar data yang terus membesar (pagination).

> Semua contoh kode di bab ini sudah dijalankan di repo ini (Go 1.27,
> AMD Ryzen 5 7500F). Angka di mesin Anda akan berbeda; yang penting
> adalah perbandingannya.

---

## Pagination

### Offset pagination: yang dipakai repo

`GET /api/v1/contacts?page=2&size=10` memakai **offset pagination**
(`internal/repository/contact_repository.go`):

```go
func (r *ContactRepository) Search(ctx context.Context, req *model.SearchContactRequest) ([]entity.Contact, int64, error) {
    var contacts []entity.Contact
    if err := r.db(ctx).Scopes(filterContact(req)).
        Order("created_at DESC").
        Offset((req.Page - 1) * req.Size).Limit(req.Size).
        Find(&contacts).Error; err != nil {
        return nil, 0, err
    }

    var total int64
    if err := r.db(ctx).Model(&entity.Contact{}).Scopes(filterContact(req)).Count(&total).Error; err != nil {
        return nil, 0, err
    }
    return contacts, total, nil
}
```

Hal-hal yang patut diperhatikan:

- **Scope dipakai ulang.** `filterContact(req)` adalah fungsi yang
  mengembalikan fungsi (closure, bab 01). Query data dan query `COUNT`
  memakai filter yang **sama persis**, jadi total tidak mungkin berbeda
  dari datanya.
- **Batas ukuran halaman divalidasi** (`validate:"min=1,max=100"` di
  `SearchContactRequest`). Tanpa batas atas, `?size=1000000` bisa membuat
  server mengambil seluruh tabel. Ini masalah performa **dan** keamanan
  (DoS).
- **Urutan wajib deterministik.** Tanpa `ORDER BY`, database boleh
  mengembalikan baris dalam urutan acak, sehingga data bisa muncul dua kali
  atau terlewat antarhalaman.
- **Metadata halaman** dihitung oleh `model.NewPageMetadata` dan dikirim
  di field `paging`:

  ```json
  { "data": [...], "paging": { "page": 2, "size": 10, "total_item": 25, "total_page": 3 } }
  ```

### Kelemahan offset

`OFFSET 100000 LIMIT 10` memaksa database **membaca lalu membuang** 100.000
baris. Semakin jauh halamannya, semakin lambat query-nya. Masalah lain:
bila ada data baru masuk saat user berpindah halaman, satu item bisa
muncul di dua halaman.

Offset tetap pilihan yang baik untuk data yang **kecil per user** (seperti
kontak di repo ini) dan UI yang butuh nomor halaman ("halaman 3 dari 12").

### Keyset (cursor) pagination

Untuk data besar atau infinite scroll, pakai **keyset pagination**:
alih-alih "lewati N baris", minta "baris setelah baris terakhir yang
sudah saya lihat".

```go
// Sketsa: sudah dicek bisa dikompilasi, tetapi tidak ada di repo.
func (r *ContactRepository) SearchAfter(ctx context.Context, userID string,
    afterCreatedAt int64, afterID string, size int) ([]entity.Contact, error) {
    var contacts []entity.Contact
    q := r.db(ctx).Where("user_id = ?", userID)
    if afterID != "" {
        q = q.Where("(created_at < ?) OR (created_at = ? AND id < ?)", afterCreatedAt, afterCreatedAt, afterID)
    }
    err := q.Order("created_at DESC, id DESC").Limit(size).Find(&contacts).Error
    return contacts, err
}
```

- `id` ditambahkan ke urutan sebagai **tie-breaker**, karena dua kontak
  bisa punya `created_at` yang sama.
- Client menerima cursor yang tidak transparan, misalnya base64 dari
  `created_at|id`, lalu mengirimkannya kembali di request berikutnya.
- Supaya cepat, perlu index yang cocok dengan urutannya, misalnya
  `(user_id, deleted_at, created_at, id)`.
- Kekurangannya: tidak bisa lompat ke "halaman 7", dan total halaman
  harus dihitung terpisah (atau tidak ditampilkan sama sekali).

| | Offset | Keyset |
|---|---|---|
| Lompat ke halaman tertentu | Bisa | Tidak bisa |
| Performa halaman jauh | Makin lambat | Konstan (dengan index) |
| Stabil saat data bertambah | Tidak | Ya |
| Cocok untuk | Tabel admin, data kecil | Feed, log, infinite scroll |

### `COUNT(*)` juga ada harganya

Setiap request daftar menjalankan **dua** query: data dan `COUNT`. Pada
tabel besar, `COUNT` bisa lebih lambat daripada query datanya. Pilihan
yang umum: hilangkan total (cukup kirim `has_next`), cache totalnya, atau
tampilkan perkiraan.

---

## Benchmark

Go punya benchmark bawaan di package `testing`. Fungsi benchmark diawali
`Benchmark` dan menerima `*testing.B`:

```go
// internal/security/perf_test.go (contoh)
func BenchmarkHashPassword(b *testing.B) {
    for b.Loop() {
        _, _ = security.HashPassword("rahasia123")
    }
}

func BenchmarkHashRefreshToken(b *testing.B) {
    plain, _, _ := security.NewRefreshToken()   // setup: tidak ikut diukur
    b.ReportAllocs()
    for b.Loop() {
        security.HashRefreshToken(plain)
    }
}
```

`b.Loop()` (Go 1.24+) mengulang badan loop sampai hasilnya stabil secara
statistik, dan memastikan kode setup di luar loop tidak ikut dihitung.

Jalankan:

```bash
go test ./internal/security/ -run '^$' -bench . -benchmem
```

`-run '^$'` mencegah unit test biasa ikut berjalan. `-benchmem` menampilkan
alokasi memori. Hasil di mesin penulis:

```
BenchmarkHashPassword-12        20   42718865 ns/op   5286 B/op   10 allocs/op
BenchmarkHashRefreshToken-12    20        140.0 ns/op  176 B/op    3 allocs/op
```

Cara membacanya: `ns/op` adalah waktu per operasi, `B/op` adalah byte yang
dialokasikan per operasi, dan `allocs/op` adalah jumlah alokasi per
operasi. Angka `-12` adalah jumlah CPU (`GOMAXPROCS`).

**bcrypt sekitar 300.000 kali lebih lambat dari SHA-256, dan memang
sengaja dibuat begitu.** Angka ini menjelaskan dua keputusan di bab 08:

- Password memakai bcrypt (~43 ms). Penyerang yang mencuri database hanya
  bisa mencoba puluhan tebakan per detik per core.
- Refresh token memakai SHA-256 (~140 ns). Token itu sudah acak penuh (256
  bit), jadi tidak bisa ditebak, dan hash cepat memungkinkan pencarian lewat
  index. Bila refresh token memakai bcrypt, setiap refresh akan memakan
  ~43 ms CPU tanpa menambah keamanan.

Angka ini juga menjelaskan mengapa endpoint login perlu **rate limit**:
setiap percobaan login menghabiskan ~43 ms CPU server, sehingga login
tanpa batas bisa dipakai untuk DoS.

### Membandingkan sebelum dan sesudah

Satu kali benchmark mudah tertipu oleh noise. Untuk membandingkan
perubahan, pakai [`benchstat`](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat):

```bash
go install golang.org/x/perf/cmd/benchstat@latest
go test -run '^$' -bench . -count 10 ./internal/security/ > old.txt
# ... ubah kode ...
go test -run '^$' -bench . -count 10 ./internal/security/ > new.txt
benchstat old.txt new.txt
```

`benchstat` memberi tahu apakah selisihnya **signifikan secara statistik**
atau hanya kebetulan.

---

## Profiling dengan pprof

Benchmark menjawab "seberapa cepat". **Profiling** menjawab "waktunya
habis di mana". Go punya profiler bawaan, `pprof`.

### Profil dari benchmark

```bash
go test ./internal/security/ -run '^$' -bench HashPassword -cpuprofile cpu.out -memprofile mem.out
go tool pprof -top cpu.out          # fungsi terberat dalam bentuk teks
go tool pprof -http=:8081 cpu.out   # UI web dengan flame graph
```

### Profil dari aplikasi yang sedang berjalan

Package `net/http/pprof` menyediakan endpoint profiling. **Jangan pernah
memasangnya di port publik**, karena endpoint ini membocorkan detail
internal dan bisa dipakai untuk membebani server. Repo ini punya tempat
yang pas, yaitu **admin server** (`internal/delivery/admin`) di port
terpisah (9090) yang tidak diekspos ke internet:

```go
// cmd/web/main.go, setelah adminServer dibuat
import "net/http/pprof"

adminServer.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))
adminServer.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
adminServer.Handle("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))
```

(Snippet ini sudah diuji dengan `httptest`: `/debug/pprof/`,
`/debug/pprof/heap`, dan `/debug/pprof/goroutine` mengembalikan 200.)

`pprof.Index` melayani semua profil bernama (`heap`, `goroutine`,
`allocs`, `block`, `mutex`), sedangkan `profile` (CPU) dan `trace` perlu
handler sendiri. Import dengan `_ "net/http/pprof"` **tidak** cukup di
sini, karena import itu hanya mendaftar ke `http.DefaultServeMux`,
sedangkan admin server memakai mux sendiri.

Lalu ambil profil saat aplikasi diberi beban:

```bash
go tool pprof -http=:8081 "http://localhost:9090/debug/pprof/profile?seconds=30"  # CPU 30 detik
go tool pprof -http=:8081 http://localhost:9090/debug/pprof/heap                  # memori
curl "http://localhost:9090/debug/pprof/goroutine?debug=1"                         # daftar goroutine
```

Profil **goroutine** sangat berguna untuk mencari *goroutine leak*: jumlah
goroutine yang terus naik biasanya berarti ada goroutine yang menunggu
channel atau context yang tidak pernah selesai (bab 03).

### Membaca hasil profiling

| Gejala | Profil | Yang dicari |
|---|---|---|
| CPU tinggi | `profile` | Fungsi dengan `flat`/`cum` terbesar |
| Memori terus naik | `heap` (`inuse_space`) | Siapa yang menahan memori |
| GC sibuk | `allocs` | Alokasi kecil yang sangat sering |
| Request lambat tapi CPU rendah | `block`, `mutex`, tracing | Menunggu lock, DB, atau jaringan |

Untuk backend, penyebab lambat yang paling sering **bukan** kode Go,
melainkan I/O: query tanpa index, N+1 query, atau pemanggilan layanan luar.
Untuk kasus seperti itu, **distributed tracing** (bab 10) lebih berguna
daripada pprof, karena menunjukkan berapa lama setiap query dan
pemanggilan dalam satu request.

---

## Hal yang memengaruhi performa di repo ini

Beberapa pengaturan yang sudah ada dan alasannya:

| Pengaturan | Lokasi | Fungsi |
|---|---|---|
| Connection pool DB (`idle: 10`, `max: 100`, `lifetime: 5m`) | `config.json`, `internal/config/database.go` | Membuka koneksi itu mahal. `max` mencegah aplikasi membanjiri MySQL, `lifetime` memastikan koneksi lama dibuang sebelum diputus oleh server atau load balancer |
| `request_timeout` | `internal/config/fiber.go` | Request yang menggantung tidak menahan goroutine dan koneksi DB selamanya |
| `body_limit` | `internal/config/fiber.go` | Body raksasa ditolak sebelum dibaca ke memori |
| Index `(user_id, deleted_at)` | `db/migrations/*_create_contacts.up.sql` | Query "kontak aktif milik user" tidak memindai seluruh tabel |
| Cache kontak | bab 13 | Mengurangi beban baca di DB |
| Batch outbox (`batch_size: 100`) | `config.json` | Relay mengirim banyak event per query, bukan satu per satu |
| Slow query log (> 200 ms) | `internal/config/database.go` | GORM mencatat query lambat sebagai warning, bahkan di production |

Untuk memeriksa apakah query memakai index, jalankan `EXPLAIN` di MySQL:

```sql
EXPLAIN SELECT * FROM contacts WHERE user_id = '...' AND deleted_at = 0 ORDER BY created_at DESC LIMIT 10;
```

Kolom `key` harus menunjukkan nama index, dan `type` tidak boleh `ALL`
(*full table scan*).

### Load testing

Benchmark mengukur satu fungsi; **load test** mengukur seluruh sistem.
Alat yang umum dipakai adalah [`k6`](https://k6.io/) atau
[`hey`](https://github.com/rakyll/hey):

```bash
hey -n 2000 -c 50 -H "Authorization: Bearer $TOKEN" http://localhost:3000/api/v1/contacts
```

Ingat bahwa rate limit global (100 request/menit per IP) akan langsung
memberi 429. Naikkan `rate_limit.global_per_minute` di lingkungan uji.
Amati p95/p99 latency di Prometheus (`http_request_duration_seconds`),
bukan rata-rata: rata-rata menyembunyikan request lambat yang justru
dirasakan user.

---

## Fuzzing

Unit test menguji input yang **terpikirkan** oleh penulisnya. **Fuzzing**
membiarkan Go membuat jutaan input acak, termasuk yang tidak terpikirkan,
untuk mencari panic atau pelanggaran aturan. Fuzzing bawaan Go tersedia
sejak Go 1.18.

### Contoh 1: invariant matematika

`model.NewPageMetadata` menghitung jumlah halaman. Aturan yang **selalu**
harus benar: jumlah halaman cukup untuk menampung semua item, tetapi tidak
berlebih.

```go
func FuzzNewPageMetadata(f *testing.F) {
    f.Add(1, 10, int64(0))      // seed corpus: contoh awal
    f.Add(3, 10, int64(25))
    f.Add(1, 100, int64(100))

    f.Fuzz(func(t *testing.T, page, size int, total int64) {
        if page < 1 || size < 1 || size > 100 || total < 0 || total > 1<<40 {
            t.Skip()             // di luar domain yang divalidasi
        }
        m := model.NewPageMetadata(page, size, total)
        if m.TotalPage*int64(size) < total {
            t.Fatalf("halaman kurang: %+v", m)
        }
        if total > 0 && (m.TotalPage-1)*int64(size) >= total {
            t.Fatalf("halaman berlebih: %+v", m)
        }
    })
}
```

```bash
go test ./internal/model/ -run '^$' -fuzz FuzzNewPageMetadata -fuzztime 10s
# fuzz: elapsed: 10s, execs: 1383348 (112031/sec) ... PASS
```

### Contoh 2: parser yang menerima input dari luar

Kandidat terbaik untuk fuzzing adalah kode yang **mem-parse input tidak
tepercaya**. Di repo ini, contohnya adalah verifikasi JWT: setiap request
membawa token yang bisa diisi apa saja oleh penyerang.

```go
func FuzzTokenParse(f *testing.F) {
    issuer := security.NewTokenIssuer([]byte("0123456789abcdef0123456789abcdef"),
        "gca", "gca-api", 15*time.Minute, time.Now)
    valid, _, _ := issuer.Issue("u1", "user")
    f.Add(valid)
    f.Add("")
    f.Add("a.b.c")
    f.Add("eyJhbGciOiJub25lIn0.eyJzdWIiOiJ1MSJ9.")   // alg "none"

    f.Fuzz(func(t *testing.T, token string) {
        claims, err := issuer.Parse(token)
        if err != nil && claims != nil {
            t.Fatalf("claims harus nil bila error, token=%q", token)
        }
        if err == nil && (claims.Subject != "u1" || claims.Role != "user") {
            t.Fatalf("token lolos dengan isi yang tidak pernah ditandatangani: %q", token)
        }
    })
}
```

Hasilnya sekitar 1,2 juta input dalam 20 detik tanpa temuan.

### Pelajaran dari percobaan pertama yang gagal

Versi pertama fuzz test di atas memakai aturan "token yang lolos harus
**persis sama** dengan `valid`". Test itu langsung gagal, bahkan pada seed
corpus, karena dua kesalahan asumsi:

1. **Fungsi fuzz dijalankan di beberapa proses worker.** Kode setup
   (`issuer.Issue`) dijalankan ulang di setiap proses, dan setiap token
   punya `jti` acak. Akibatnya `valid` di worker berbeda dari seed yang
   dicatat proses utama.
2. **Representasi berbeda tidak berarti isi berbeda.** Dua string base64
   bisa ter-decode menjadi byte yang sama. Token yang "terlihat berbeda"
   tetapi isinya sama dan tanda tangannya sah tidak berbahaya.

Aturan yang benar bukan "string harus sama", melainkan **"isi token yang
diterima harus persis isi yang ditandatangani"**. Merumuskan *property*
yang benar adalah bagian tersulit dari fuzzing, dan juga yang paling
mendidik.

### Bila fuzzer menemukan bug

Go menyimpan input yang gagal di
`testdata/fuzz/FuzzXxx/<hash>`. **Commit file itu.** Setelah itu input
tersebut ikut dijalankan setiap kali `go test` biasa berjalan (tanpa
`-fuzz`), sehingga menjadi regression test permanen.

Fuzzing tidak dijalankan di CI biasa karena tidak pernah "selesai".
Jalankan secara berkala (misalnya beberapa menit di job malam), atau
manual setelah mengubah parser.

---

## Latihan

1. Salin dua benchmark di atas ke `internal/security/perf_test.go` dan
   jalankan. Ubah `bcrypt.DefaultCost` (10) menjadi 12 untuk sementara.
   Berapa kali lebih lambat? Mengapa setiap kenaikan 1 membuatnya dua kali
   lebih lambat?
2. Pasang pprof di admin server, jalankan `hey` ke endpoint login, lalu
   ambil profil CPU 30 detik. Fungsi apa yang paling berat? Apakah sesuai
   dugaan Anda setelah membaca bagian benchmark?
3. Jalankan `EXPLAIN` untuk query pencarian kontak dengan filter `name`.
   Apakah index dipakai? Mengapa `LIKE '%budi%'` tidak bisa memakai index
   B-tree biasa? Alternatif apa yang tersedia (full-text index, mesin
   pencari)?
4. Implementasikan `GET /api/v1/contacts?cursor=...` dengan keyset
   pagination. Rancang format cursor-nya. Mengapa cursor sebaiknya tidak
   transparan (base64, atau ditandatangani) alih-alih `?after_id=...`?
5. Tulis fuzz test untuk `security.HashRefreshToken`. Property apa yang
   masuk akal? (Petunjuk: panjang output, karakter yang dipakai, dan
   determinisme.)
6. Cari N+1 query: buat endpoint yang mengembalikan kontak beserta
   alamatnya. Aktifkan log SQL GORM (`log.level: debug`) dan hitung jumlah
   query. Perbaiki dengan `Preload` atau satu query `IN (...)`.
