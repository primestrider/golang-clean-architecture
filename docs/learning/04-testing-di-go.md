# 04. Testing di Go

Go punya tool test bawaan: file berakhiran `_test.go`, fungsi
`func TestXxx(t *testing.T)`, dan perintah `go test`.

```bash
go test ./internal/...                               # unit test
go test -run TestContactGet -v ./internal/usecase/   # satu kelompok test, output detail
go test -tags integration -count=1 ./test/...        # integration test (butuh Docker)
```

## Tiga jenis test di repo ini

| Jenis | Lokasi | Dependency | Kecepatan |
|---|---|---|---|
| Unit test | `internal/**/xxx_test.go` | Mock dan fake | Milidetik |
| Integration test | `test/*_test.go` (tag `integration`) | Container sungguhan (MySQL, Redis, Kafka, S3, Mailpit) | Detik |
| Uji asap (manual) | `docker compose` + `curl` / `test/manual.http` | Stack lengkap | Menit |

## Bentuk test: Arrange, Act, Assert

Setiap test di repo ini mengikuti tiga bagian yang dipisah baris kosong:

```go
// internal/usecase/contact_usecase_test.go
func TestContactGet_NotOwnedIsNotFound(t *testing.T) {
    f := newContactFixture(t)                                    // Arrange
    id := "6f1c1f0e-4b7a-4c64-9d7e-6a0b7a1d2c3e"
    f.cache.EXPECT().Get(mock.Anything, mock.Anything, mock.Anything).Return(false, nil)
    f.contacts.EXPECT().FindByIDAndUserID(mock.Anything, id, "u1").Return(nil, apperror.NotFound("contact"))

    _, err := f.uc.Get(context.Background(), &model.GetContactRequest{UserID: "u1", ID: id}) // Act

    assert.Equal(t, apperror.CodeNotFound, codeOf(err))          // Assert
}
```

Nama test menjelaskan perilaku: `TestApaYangDiuji_KondisiDanHasil`.

## testify: assert dan require

- `assert.Equal(...)`: bila gagal, test tetap lanjut (berguna untuk
  melaporkan beberapa kegagalan sekaligus).
- `require.NoError(...)`: bila gagal, test langsung berhenti (dipakai bila
  langkah berikutnya bergantung pada langkah ini).

## Mock dengan mockery

Interface di `internal/usecase/ports.go` di-generate menjadi mock di
`internal/usecase/mocks` dengan `make mocks` (konfigurasinya di `.mockery.yaml`).

```go
f.contacts.EXPECT().
    FindByIDAndUserID(mock.Anything, "c1", "u1").
    Return(&entity.Contact{ID: "c1"}, nil)
```

- Mock yang dibuat dengan `mocks.NewContactRepository(t)` **otomatis gagal**
  bila ada pemanggilan yang tidak diharapkan, atau bila ekspektasi tidak
  terpenuhi.
- `mock.MatchedBy(func(x T) bool {...})` memeriksa isi argumen.
- `RunAndReturn(func(...) ...)` menjalankan logika kustom, misalnya
  memastikan event diterbitkan **di dalam** transaksi:

```go
f.events.EXPECT().Publish(mock.Anything, mock.Anything).
    RunAndReturn(func(_ context.Context, events ...model.Event) error {
        assert.True(t, f.tx.inTx, "event must be published inside the transaction")
        return nil
    })
```

## Fake: implementasi palsu yang sederhana

Tidak semua hal perlu mock. Fake berupa struct kecil sering lebih jelas:

```go
// internal/usecase/testhelpers_test.go
type fakeTransactor struct{ inTx bool }

func (f *fakeTransactor) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
    f.inTx = true
    defer func() { f.inTx = false }()
    return fn(ctx)
}
```

Contoh lain: `fakeMailer` di `notification_usecase_test.go`, serta `fakeRepo`
dan `fakeDispatcher` di `internal/outbox/relay_test.go`.

## Waktu yang bisa dikendalikan

Kode yang bergantung pada waktu sulit diuji. Solusinya, suntikkan fungsi `now`:

```go
// internal/security/jwt.go
func NewTokenIssuer(secret []byte, issuer, audience string, ttl time.Duration, now func() time.Time) *TokenIssuer
```

Test lalu bisa menguji token kedaluwarsa tanpa menunggu 15 menit
(`TestParse_RejectsExpired`). Pola yang sama ada di `scheduler.Jobs.Now` dan
`ipLimiter.now`.

## Integration test dengan testcontainers

`test/testsupport` menyalakan container Docker sungguhan dari dalam test:

```go
// test/main_test.go
func TestMain(m *testing.M) {
    os.Exit(testsupport.RunMain(m))   // satu container MySQL untuk seluruh package
}

// test/contact_test.go
func TestGetContact_OtherUsersContactIsNotFound(t *testing.T) {
    app, _, token := loggedIn(t)                      // aplikasi lengkap + user login
    other := app.CreateUser(t, "other@example.com", "rahasia123")
    contact := app.CreateContact(t, other.ID, "Rahasia")

    resp, _ := app.Do(t, http.MethodGet, "/api/v1/contacts/"+contact.ID, nil, token)

    assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
```

Prinsip penting:

- **Setiap test independen.** `NewWebApp` mengosongkan semua tabel, sehingga
  urutan test tidak berpengaruh.
- **Build tag `//go:build integration`** membuat test ini hanya berjalan
  bila diminta.
- `GCA_TEST_LOG=1` menampilkan log aplikasi saat men-debug test yang gagal.

## Test untuk kasus konkurensi

`TestRefresh_ConcurrentSameToken` (`test/auth_test.go`) mengirim dua refresh
bersamaan dengan token yang sama, lalu memastikan tepat satu yang berhasil.
Bug semacam ini tidak akan pernah terlihat di test berurutan.

## Membuktikan test benar-benar menguji sesuatu

Test yang langsung hijau belum tentu benar. Repo ini memakai **mutation
check**: sengaja merusak kode, memastikan test-nya gagal, lalu
mengembalikan kodenya. Contoh yang dilakukan selama pengembangan:

- Menghapus `FOR UPDATE` di `FindByHashForUpdate` membuat
  `TestRefresh_ConcurrentSameToken` gagal 8 dari 8 kali.
- Mengubah filter kepemilikan `AND` menjadi `OR` membuat test akses kontak
  user lain gagal.

Prinsip TDD yang dipakai: **tulis test dulu, lihat test itu gagal, baru
tulis kode**.

## Latihan

1. Tulis unit test `TestAddressUpdate_StaleVersionReturnsConflict` memakai
   mock `AddressRepository` yang mengembalikan `apperror.Conflict`.
2. Tulis integration test yang memastikan `GET /api/v1/contacts?size=101`
   mengembalikan 400 dengan error di field `size`.
3. Lakukan mutation check: hapus `deleteObject(ctx, *oldKey)` di
   `AvatarUseCase.Upload`. Test mana yang gagal?
