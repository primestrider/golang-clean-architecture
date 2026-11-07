# 07. Database

## Migrasi: skema sebagai kode

Skema database tidak dibuat manual. Setiap perubahan adalah pasangan file di
`db/migrations`:

```
000002_create_contacts.up.sql     ← menerapkan perubahan
000002_create_contacts.down.sql   ← membatalkannya
```

- Dijalankan dengan golang-migrate (`make migrate-up`, atau service `migrate`
  di docker-compose).
- `TestMigrations_UpDown` memastikan semua migrasi bisa di-*down* lalu di-*up*
  ulang.
- **Aturan emas:** jangan pernah mengubah file migrasi yang sudah diterapkan
  di production. Buat migrasi baru.

Hal yang patut diperhatikan di skema repo ini:

- `id CHAR(36)` berisi UUID: tidak bisa ditebak dan aman dibuat di banyak server.
- `created_at BIGINT` berisi epoch milidetik: tidak ada masalah zona waktu.
- `UNIQUE KEY (email, deleted_at)`: email unik di antara user aktif, tetapi
  email user yang sudah dihapus boleh dipakai lagi.
- Foreign key menjaga integritas (alamat selalu punya kontak).

## GORM dalam satu menit

GORM memetakan struct ke tabel:

```go
type Contact struct {
    ID        string `gorm:"column:id;primaryKey"`
    UserID    string `gorm:"column:user_id"`
    Version   int    `gorm:"column:version;default:1"`
    DeletedAt soft_delete.DeletedAt `gorm:"column:deleted_at;softDelete:milli"`
}

db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).Take(&contact)
```

Gunakan **selalu** placeholder `?`, jangan menyambung string SQL. Dengan
placeholder, nilai dikirim terpisah dari query sehingga SQL injection tidak
mungkin terjadi. Contohnya filter pencarian di
`internal/repository/contact_repository.go`.

## Repository

Repository adalah satu-satunya tempat query ditulis:

```go
// generik, untuk semua entity
func (r *Repository[T]) FindByID(ctx context.Context, id string) (*T, error)

// khusus kontak
func (r *ContactRepository) FindByIDAndUserID(ctx context.Context, id, userID string) (*entity.Contact, error)
```

Konvensi error:

- Data tidak ada → `apperror.NotFound("contact")` (diterjemahkan di `take()`).
- Email duplikat → `apperror.Conflict`, dideteksi lewat `gorm.ErrDuplicatedKey`
  karena GORM dibuka dengan `TranslateError: true`.
- Error lain dikembalikan apa adanya dan menjadi 500.

**Kepemilikan data dicek di query**, bukan setelahnya. Kontak milik user
lain dilaporkan "not found", bukan "forbidden", supaya keberadaannya tidak
bocor.

## Transaksi lewat context

Masalahnya: use case perlu beberapa operasi dalam satu transaksi (simpan
kontak, tulis audit, tulis outbox), tetapi use case tidak boleh mengenal GORM.

Solusinya `Transactor`:

```go
err := c.tx.WithinTransaction(ctx, func(ctx context.Context) error {
    if err := c.contacts.Create(ctx, contact); err != nil {
        return err                           // rollback
    }
    if err := c.audit.Record(ctx, ...); err != nil {
        return err                           // rollback
    }
    return publish(ctx, c.events, "contact.created", ...)
})                                            // nil → commit
```

- Transaksi disimpan di `ctx`, dan setiap repository mengambilnya lewat
  `repository.DB(ctx, base)`.
- Di unit test, `fakeTransactor` cukup memanggil `fn(ctx)`.
- Operasi baca tidak membuka transaksi, sehingga lebih ringan.

## Soft delete

Data tidak benar-benar dihapus; `deleted_at` diisi waktu penghapusan. Plugin
`gorm.io/plugin/soft_delete` otomatis:

- menambahkan `WHERE deleted_at = 0` di setiap query,
- mengubah `DELETE` menjadi `UPDATE ... SET deleted_at = <now>`.

Manfaatnya: data bisa dipulihkan dan jejak audit tetap utuh. Hapus kontak
juga men-soft-delete alamatnya **dalam transaksi yang sama**
(`ContactUseCase.Delete`).

## Optimistic locking

Skenario: Ani dan Budi membuka kontak yang sama. Ani menyimpan, lalu Budi
menyimpan. Tanpa perlindungan, perubahan Ani hilang diam-diam (*lost update*).

Solusinya kolom `version`:

```sql
UPDATE contacts SET ..., version = 3 WHERE id = ? AND version = 2
```

- Client mengirim `version` terakhir yang ia baca.
- Bila 0 baris berubah: data sudah dihapus (404) atau sudah diubah orang
  lain (409). Client lalu harus memuat ulang.
- Implementasinya `Repository[T].UpdateWithVersion` di
  `internal/repository/repository.go`.

Mengapa "optimistic"? Kita *berharap* tidak ada konflik dan baru memeriksa
saat menyimpan, tanpa mengunci baris selama user mengedit.

## Pessimistic locking: `SELECT ... FOR UPDATE`

Untuk refresh token, konflik **pasti** terjadi bila dua request datang
bersamaan, jadi barisnya dikunci:

```go
r.db(ctx).Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate}).
    Where("token_hash = ?", hash).Take(&token)
```

Request kedua menunggu sampai transaksi pertama selesai, lalu melihat token
sudah dicabut. Detailnya ada di [bab 08](08-autentikasi-keamanan.md).

## Pelajaran nyata: urutan assignment di MySQL

Saat pengembangan ditemukan bug di `MarkFailedAttempt`. MySQL mengevaluasi
`SET a = ..., b = ...` **dari kiri ke kanan**, dan GORM mengurutkan key map
secara alfabetis. Akibatnya `status` dihitung dari `attempts` yang sudah
dinaikkan. Perbaikannya memakai SQL eksplisit dengan urutan yang benar.
Test integrasi dengan database sungguhan menangkap bug ini; mock tidak akan
pernah menangkapnya.

## Latihan

1. Tulis migrasi `000008` yang menambah kolom `company VARCHAR(100) NULL` ke
   `contacts`, lengkap dengan file `.down.sql`. Perbarui entity, model,
   converter, dan test-nya.
2. Mengapa `UNIQUE(email, deleted_at)` bekerja dengan `deleted_at = 0`,
   tetapi tidak bila soft delete memakai `NULL`?
3. Upload avatar memakai `SetAvatarKey` (hanya satu kolom dan menaikkan
   version) alih-alih `Save` seluruh baris. Jelaskan skenario bug yang dicegah.
