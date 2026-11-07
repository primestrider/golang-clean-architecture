# 00. Pendahuluan

## Menyiapkan lingkungan

1. **Go 1.27.3**: <https://go.dev/dl/>. Cek dengan `go version`.
2. **Docker Desktop**: untuk menjalankan MySQL dan dependency lain, serta
   integration test.
3. **Editor**: VS Code dengan ekstensi Go, atau GoLand. Ekstensi Go
   memasang `gopls` (language server) yang memberi autocomplete, "go to
   definition", dan menampilkan doc comment saat kursor diarahkan ke simbol.
4. **Tool pengembangan** (opsional): `make tools` memasang golangci-lint,
   mockery, govulncheck, buf, dan plugin protobuf.

Jalankan aplikasi sekali supaya Anda punya gambaran utuh:

```bash
docker compose up -d --build
curl -X POST localhost:3000/api/v1/users \
  -H 'Content-Type: application/json' \
  -d '{"email":"saya@example.com","password":"rahasia123","name":"Saya"}'
```

Buka <http://localhost:3000/docs> untuk mencoba semua endpoint lewat Swagger UI.

## Peta repo

```
cmd/            ← titik masuk program (package main)
  web/            API HTTP (+ gRPC opsional)
  worker/         pekerjaan latar belakang
  cli/            perintah administrasi
api/            ← kontrak API: openapi.yaml dan file .proto
db/migrations/  ← perubahan skema database (SQL)
internal/       ← kode aplikasi (tidak bisa diimpor dari luar module)
  config/         merangkai semua komponen (composition root)
  delivery/       pintu masuk: HTTP, gRPC, Kafka, admin
  usecase/        logika bisnis
  repository/     akses database
  gateway/        akses sistem luar: Redis, S3, SMTP
  entity/         struct tabel database
  model/          struct request, response, dan event
  ...
test/           ← integration test
```

Folder `internal/` punya arti khusus di Go: package di dalamnya **hanya bisa
diimpor oleh kode di dalam module yang sama**. Ini cara Go menyembunyikan
detail implementasi dari pihak luar.

## Cara membaca kode Go di repo ini

1. **Mulai dari `main`.** Buka `cmd/web/main.go`. Fungsi `run()` menunjukkan
   urutan hidup aplikasi: muat config, buat koneksi database, rangkai
   komponen, jalankan server, tunggu sinyal, lalu shutdown.
2. **Ikuti wiring.** `config.BootstrapWeb` (`internal/config/web.go`)
   menunjukkan bagaimana repository, use case, dan controller dirangkai.
   Baca dari atas ke bawah; urutannya mengikuti arah dependency.
3. **Ikuti satu request.** Contoh `POST /api/v1/contacts`:
   - `internal/delivery/http/route/route.go` mendaftarkan route ke controller.
   - `internal/delivery/http/contact_controller.go` mem-parse body dan
     memanggil use case.
   - `internal/usecase/contact_usecase.go` melakukan validasi, menjalankan
     transaksi, menulis audit, dan menerbitkan event.
   - `internal/repository/contact_repository.go` menjalankan query ke database.
4. **Baca doc comment-nya.** Setiap simbol punya komentar `// NamaSimbol ...`
   yang menjelaskan *apa* dan *mengapa*. Di terminal:

   ```bash
   go doc ./internal/usecase ContactUseCase
   go doc -all ./internal/apperror
   ```

## Perintah Go yang paling sering dipakai

| Perintah | Fungsi |
|---|---|
| `go run ./cmd/web` | Kompilasi lalu jalankan |
| `go build ./...` | Kompilasi semua package (cek error) |
| `go test ./internal/...` | Jalankan unit test |
| `go test -run TestNama -v ./internal/usecase/` | Jalankan satu test dengan output detail |
| `go vet ./...` | Cari pola kode yang mencurigakan |
| `gofmt -w .` | Format kode (gaya Go itu baku, tidak diperdebatkan) |
| `go mod tidy` | Rapikan daftar dependency di `go.mod` |
| `go doc <pkg> <simbol>` | Baca dokumentasi |

## Latihan

1. Jalankan `go doc ./internal/apperror` dan sebutkan semua `Code` yang ada.
2. Dari `cmd/worker/main.go`, tuliskan urutan langkah saat worker dimatikan.
3. Telusuri `PATCH /api/v1/users/_current` dari route sampai repository.
   File apa saja yang dilewati?
