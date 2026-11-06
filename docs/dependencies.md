# Dependency Proyek: `go.mod` dan `go.sum`

Dokumen ini menjelaskan cara Go mengelola dependency di proyek ini, serta fungsi setiap package yang terpasang.

## Daftar Isi

- [Apa itu `go.mod`](#apa-itu-gomod)
- [Apa itu `go.sum`](#apa-itu-gosum)
- [Dependency langsung](#dependency-langsung)
- [Dependency tidak langsung (indirect)](#dependency-tidak-langsung-indirect)
- [Perintah yang sering dipakai](#perintah-yang-sering-dipakai)

---

## Apa itu `go.mod`

`go.mod` adalah file definisi **module** Go. Isinya identitas proyek dan daftar dependency beserta versinya. File ini ditulis manusia dan dirawat oleh perintah `go`.

```go
module golang-clean-architecture   // nama module, dipakai sebagai prefix import

go 1.26.0                          // versi minimum bahasa Go

require (                          // dependency langsung
	github.com/gofiber/fiber/v3 v3.6.1
	...
)

require (                          // dependency tidak langsung
	github.com/valyala/fasthttp v1.77.0 // indirect
	...
)
```

| Directive | Fungsi |
|---|---|
| `module` | Nama module. Package internal di-import dengan prefix ini, misalnya `golang-clean-architecture/internal/config`. |
| `go` | Versi minimum Go untuk mem-build proyek. Nilai ini juga menentukan fitur bahasa yang boleh dipakai. Contohnya, `sync.WaitGroup.Go` yang dipakai di `cmd/worker/main.go` butuh Go ≥ 1.25. |
| `require` | Daftar module beserta versi minimum yang dibutuhkan. |
| `// indirect` | Module ini tidak di-import langsung oleh kode proyek, tapi dibutuhkan oleh dependency lain. |

### Format versi

Go memakai [Semantic Versioning](https://semver.org) dengan format `vMAJOR.MINOR.PATCH`:

- **PATCH** (`v2.52.9` → `v2.52.16`): perbaikan bug atau keamanan, tanpa perubahan API.
- **MINOR** (`v1.30.1` → `v1.31.2`): fitur baru yang tetap kompatibel.
- **MAJOR** (`v2` → `v3`): ada breaking change. Untuk v2 ke atas, nomor major menjadi bagian dari path import, misalnya `github.com/gofiber/fiber/v2` dan `github.com/gofiber/fiber/v3`. Karena itu kedua versi dianggap module yang berbeda.
- **Pseudo-version** (`v0.0.0-20250401214520-65e299d6c5c9`): dipakai ketika module tidak punya tag rilis. Formatnya waktu commit (UTC) diikuti hash commit.

### Minimal Version Selection (MVS)

Jika beberapa dependency membutuhkan versi berbeda dari module yang sama, Go memilih **versi tertinggi di antara versi minimum yang diminta**. Go tidak otomatis memakai versi terbaru yang ada di internet. Akibatnya build bersifat *reproducible*: hasilnya sama selama `go.mod` tidak berubah.

---

## Apa itu `go.sum`

`go.sum` berisi **checksum kriptografis** untuk setiap module yang pernah diunduh. File ini dibuat otomatis dan **tidak boleh diedit manual**.

```
github.com/IBM/sarama v1.61.1 h1:I59MWPHQUWqJNdRpsDUcbeCriog8SxjQaPfHNWxidEg=
github.com/IBM/sarama v1.61.1/go.mod h1:dITlGHIiCQL/maGtBfDHNMDvyWgC9Ww//8pmlsU3RUs=
```

Setiap module biasanya punya dua baris:

| Baris | Isi yang di-hash |
|---|---|
| `<module> <versi> h1:...` | Seluruh isi source code module (file zip). |
| `<module> <versi>/go.mod h1:...` | Hanya file `go.mod` milik module tersebut. Dipakai untuk menghitung graph dependency tanpa mengunduh source lengkap. |

`h1:` menandakan algoritma hash SHA-256 yang di-encode base64.

### Kenapa `go.sum` penting

1. **Integritas.** Setiap kali module diunduh, Go mencocokkan hash-nya dengan `go.sum`. Jika berbeda, build gagal dengan error `checksum mismatch`. Ini melindungi proyek dari kode yang diubah diam-diam.
2. **Verifikasi global.** Saat module pertama kali ditambahkan, hash-nya juga dicocokkan dengan database checksum publik di `sum.golang.org`.
3. **Reproducible.** Semua developer dan server CI dijamin memakai source code yang persis sama.

> **`go.mod` dan `go.sum` wajib di-commit ke Git.** `go.sum` juga bisa memuat baris dari module yang tidak lagi dipakai langsung. Itu normal. Jalankan `go mod tidy` untuk merapikannya.

---

## Dependency langsung

Package berikut di-import langsung oleh kode proyek.

| Package | Versi | Fungsi | Dipakai di |
|---|---|---|---|
| [`github.com/gofiber/fiber/v3`](https://github.com/gofiber/fiber) | v3.6.1 | Web framework HTTP berbasis fasthttp, dengan API mirip Express.js. Mengurus routing, middleware (termasuk `recover`), request/response, error handler, dan graceful shutdown lewat `ListenConfig.GracefulContext`. | `internal/config/fiber.go`, `cmd/web/main.go`, `internal/delivery/http/` |
| [`gorm.io/gorm`](https://gorm.io) | v1.31.2 | ORM (Object Relational Mapper). Memetakan struct Go ke tabel database dan menangani query serta transaksi. | `internal/repository/`, `internal/usecase/` |
| [`gorm.io/driver/mysql`](https://github.com/go-gorm/mysql) | v1.6.0 | Driver MySQL untuk GORM. | `internal/config/gorm.go` |
| [`github.com/IBM/sarama`](https://github.com/IBM/sarama) | v1.61.1 | Client Apache Kafka. Dipakai untuk mengirim event (producer) dan membaca event (consumer group). | `internal/config/kafka.go`, `internal/gateway/messaging/`, `internal/delivery/messaging/` |
| [`github.com/spf13/viper`](https://github.com/spf13/viper) | v1.21.0 | Pembaca konfigurasi dari `config.json`. Nilainya bisa ditimpa environment variable, misalnya `DATABASE_PASSWORD`. | `internal/config/viper.go` |
| [`github.com/go-playground/validator/v10`](https://github.com/go-playground/validator) | v10.30.5 | Validasi struct lewat tag, misalnya `validate:"required,max=100"`. | `internal/config/validator.go`, `internal/model/`, `internal/usecase/` |
| [`golang.org/x/crypto`](https://pkg.go.dev/golang.org/x/crypto) | v0.58.0 | Library kriptografi resmi tim Go. Proyek ini memakai `bcrypt` untuk hashing password. | `internal/usecase/user_usecase.go` |
| [`github.com/google/uuid`](https://github.com/google/uuid) | v1.6.0 | Membuat UUID. Dipakai untuk token login dan ID data. | `internal/usecase/` |
| [`github.com/stretchr/testify`](https://github.com/stretchr/testify) | v1.12.1 | Helper testing (`assert`, `require`). | `test/` |

Untuk logging, proyek ini memakai [`log/slog`](https://pkg.go.dev/log/slog) dari standard library Go, jadi tidak tercantum di `go.mod`. Logger dibuat di `internal/config/logger.go` dengan output JSON. Levelnya diatur lewat `log.level` di `config.json` (`debug`, `info`, `warn`, atau `error`) atau env `LOG_LEVEL`.

---

## Dependency tidak langsung (indirect)

Module berikut tidak di-import langsung oleh kode proyek, tapi dibutuhkan oleh dependency di atas. Kolom "Dibawa oleh" menunjukkan dependency langsung yang membutuhkannya.

### HTTP (dibawa oleh Fiber)

| Package | Versi | Fungsi |
|---|---|---|
| `github.com/valyala/fasthttp` | v1.77.0 | HTTP server berperforma tinggi yang menjadi fondasi Fiber. |
| `github.com/valyala/bytebufferpool` | v1.0.0 | Pool buffer byte untuk mengurangi alokasi memori. |
| `github.com/klauspost/compress` | v1.20.3 | Kompresi gzip, deflate, dan zstd. Juga dipakai Sarama untuk kompresi pesan Kafka. |
| `github.com/molecule-man/go-brrr` | v1.3.1 | Kompresi Brotli untuk respons HTTP. |
| `github.com/mattn/go-colorable` | v0.1.16 | Output terminal berwarna, termasuk di Windows. |
| `github.com/mattn/go-isatty` | v0.0.24 | Mendeteksi apakah output mengarah ke terminal. |
| `github.com/gofiber/utils/v2` | v2.6.2 | Kumpulan helper internal Fiber (konversi string/byte, parsing). |
| `github.com/gofiber/schema` | v1.8.8 | Decoder form dan query string ke struct, dipakai oleh `Bind()`. |
| `github.com/tinylib/msgp` | v1.6.5 | Encoder/decoder MessagePack, untuk binding body `application/vnd.msgpack`. |
| `github.com/philhofer/fwd` | v1.2.0 | Buffered reader/writer, dibawa oleh `msgp`. |

### Database (dibawa oleh GORM dan driver MySQL)

| Package | Versi | Fungsi |
|---|---|---|
| `github.com/go-sql-driver/mysql` | v1.10.1 | Driver MySQL standar untuk `database/sql`, dipakai di balik `gorm.io/driver/mysql`. |
| `filippo.io/edwards25519` | v1.2.0 | Kriptografi kurva Ed25519, dibutuhkan driver MySQL untuk beberapa metode autentikasi. |
| `github.com/jinzhu/inflection` | v1.0.0 | Mengubah kata tunggal menjadi jamak, misalnya untuk nama tabel otomatis (`User` → `users`). |
| `github.com/jinzhu/now` | v1.1.5 | Helper manipulasi waktu. |

### Kafka (dibawa oleh Sarama)

| Package | Versi | Fungsi |
|---|---|---|
| `github.com/eapache/go-resiliency` | v1.7.0 | Pola ketahanan seperti retrier dan circuit breaker. |
| `github.com/pierrec/lz4/v4` | v4.1.33 | Kompresi LZ4 untuk pesan Kafka. |
| `github.com/rcrowley/go-metrics` | pseudo-version | Pengumpulan metrik internal client Kafka. |
| `github.com/hashicorp/go-uuid` | v1.0.4 | Pembuatan UUID, dibawa oleh library Kerberos. |
| `github.com/jcmturner/gokrb5/v8` | v8.4.4 | Autentikasi Kerberos (SASL/GSSAPI) untuk Kafka. |
| `github.com/jcmturner/aescts/v2` | v2.0.0 | Enkripsi AES-CTS untuk Kerberos. |
| `github.com/jcmturner/dnsutils/v2` | v2.0.0 | Utilitas DNS untuk Kerberos. |
| `github.com/jcmturner/gofork` | v1.7.6 | Fork beberapa package standar Go yang dibutuhkan Kerberos. |
| `github.com/jcmturner/rpc/v2` | v2.0.3 | Protokol RPC untuk Kerberos. |
| `golang.org/x/net` | v0.62.0 | Library jaringan tambahan dari tim Go. |

### Konfigurasi (dibawa oleh Viper)

| Package | Versi | Fungsi |
|---|---|---|
| `github.com/fsnotify/fsnotify` | v1.10.1 | Memantau perubahan file, untuk fitur reload config. |
| `github.com/go-viper/mapstructure/v2` | v2.5.0 | Mengubah `map` menjadi struct. |
| `github.com/pelletier/go-toml/v2` | v2.4.3 | Parser file TOML. |
| `go.yaml.in/yaml/v3` | v3.0.5 | Parser file YAML. |
| `github.com/sagikazarmark/locafero` | v0.12.0 | Pencarian lokasi file konfigurasi. |
| `github.com/spf13/afero` | v1.15.0 | Abstraksi filesystem. |
| `github.com/spf13/cast` | v1.10.0 | Konversi tipe data, misalnya string ke int. |
| `github.com/spf13/pflag` | v1.0.10 | Parsing flag command line. |
| `github.com/subosito/gotenv` | v1.6.0 | Parser file `.env`. |

### Validasi (dibawa oleh Validator)

| Package | Versi | Fungsi |
|---|---|---|
| `github.com/gabriel-vasile/mimetype` | v1.4.16 | Deteksi tipe MIME file, untuk validasi upload. |
| `github.com/go-playground/locales` | v0.14.2 | Data lokal (bahasa, format angka). |
| `github.com/go-playground/universal-translator` | v0.18.2 | Penerjemah pesan error validasi. |
| `github.com/leodido/go-urn` | v1.5.0 | Parser URN untuk validasi tag `urn`. |

### Umum

| Package | Versi | Dibawa oleh | Fungsi |
|---|---|---|---|
| `golang.org/x/sys` | v0.49.0 | banyak | Akses system call OS tingkat rendah. |
| `golang.org/x/text` | v0.43.0 | GORM, Validator | Pemrosesan teks Unicode. |
| `github.com/davecgh/go-spew` | pseudo-version | Testify | Mencetak isi struct secara detail saat test gagal. |

---

## Perintah yang sering dipakai

| Perintah | Fungsi |
|---|---|
| `go mod tidy` | Menambah dependency yang kurang, menghapus yang tidak dipakai, dan merapikan `go.sum`. |
| `go get <module>@<versi>` | Menambah atau mengubah versi dependency, misalnya `go get github.com/gofiber/fiber/v3@v3.6.1`. |
| `go get -u ./...` | Update semua dependency ke versi minor atau patch terbaru. |
| `go list -m -u all` | Melihat dependency yang punya versi baru. |
| `go mod why -m <module>` | Menjelaskan kenapa sebuah module dibutuhkan, termasuk rantai import-nya. |
| `go mod graph` | Menampilkan seluruh graph dependency. |
| `go mod verify` | Memastikan module di cache lokal cocok dengan `go.sum`. |
| `go run golang.org/x/vuln/cmd/govulncheck@latest ./...` | Memindai kerentanan keamanan yang benar-benar terpanggil oleh kode. |

Alur update yang disarankan:

```bash
go get -u ./...
go mod tidy
go build ./... && go vet ./...
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
go test ./...
```
