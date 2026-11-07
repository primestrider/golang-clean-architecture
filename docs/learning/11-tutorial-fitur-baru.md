# 11. Tutorial: Membangun Fitur "Catatan Kontak"

Di bab ini Anda membangun satu fitur lengkap mengikuti pola repo:
**catatan bebas per kontak** ("suka kopi", "ulang tahun 12 Mei").

| Endpoint | Fungsi |
|---|---|
| `POST /api/v1/contacts/{contactId}/notes` | Tambah catatan |
| `GET /api/v1/contacts/{contactId}/notes` | Daftar catatan |
| `DELETE /api/v1/contacts/{contactId}/notes/{noteId}` | Hapus catatan |

> Semua kode di bab ini sudah diverifikasi: dikompilasi, lolos lint, dan
> lolos unit test serta integration test-nya. Kodenya **tidak** disertakan
> di repo supaya Anda bisa mengerjakannya sendiri.

Urutan pengerjaan mengikuti arah dependency, dari dalam ke luar:

```
migrasi → entity → model → port → repository → use case (+ unit test)
        → controller → route → wiring → OpenAPI → integration test
```

Buat branch baru dulu: `git checkout -b feature/contact-notes`.

---

## Langkah 1: Migrasi

Buat `db/migrations/000008_create_contact_notes.up.sql`:

```sql
-- Catatan bebas milik sebuah kontak.
CREATE TABLE contact_notes
(
    id         CHAR(36) NOT NULL,
    contact_id CHAR(36) NOT NULL,
    body       TEXT     NOT NULL,
    created_at BIGINT   NOT NULL,
    updated_at BIGINT   NOT NULL,
    deleted_at BIGINT   NOT NULL DEFAULT 0,
    PRIMARY KEY (id),
    KEY idx_contact_notes_contact (contact_id, deleted_at),
    CONSTRAINT fk_contact_notes_contact FOREIGN KEY (contact_id) REFERENCES contacts (id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;
```

Dan `000008_create_contact_notes.down.sql`:

```sql
DROP TABLE IF EXISTS contact_notes;
```

Index `(contact_id, deleted_at)` mempercepat query "semua catatan aktif
milik kontak X", yaitu query yang paling sering dijalankan fitur ini.

`TestMigrations_UpDown` (`test/migrations_test.go`) memeriksa daftar tabel
secara persis, jadi tambahkan `"contact_notes"` ke daftar itu. Test seperti
ini memaksa Anda sadar setiap kali skema berubah.

## Langkah 2: Entity

`internal/entity/note_entity.go`:

```go
package entity

import "gorm.io/plugin/soft_delete"

// Note adalah catatan bebas milik sebuah Contact.
type Note struct {
	ID        string                `gorm:"column:id;primaryKey"`
	ContactID string                `gorm:"column:contact_id"`
	Body      string                `gorm:"column:body"`
	CreatedAt int64                 `gorm:"column:created_at;autoCreateTime:milli"`
	UpdatedAt int64                 `gorm:"column:updated_at;autoCreateTime:milli;autoUpdateTime:milli"`
	DeletedAt soft_delete.DeletedAt `gorm:"column:deleted_at;softDelete:milli"`
}

// TableName memberi tahu GORM nama tabel untuk entity ini.
func (n *Note) TableName() string {
	return "contact_notes"
}
```

## Langkah 3: Model dan converter

`internal/model/note_model.go`:

```go
package model

// NoteResponse adalah representasi catatan yang dikirim ke client.
type NoteResponse struct {
	ID        string `json:"id"`
	Body      string `json:"body"`
	CreatedAt int64  `json:"created_at"`
}

// CreateNoteRequest adalah input pembuatan catatan. UserID dan ContactID
// diisi controller dari token dan URL, bukan dari body.
type CreateNoteRequest struct {
	UserID    string `json:"-" validate:"required"`
	ContactID string `json:"-" validate:"required,uuid"`
	Body      string `json:"body" validate:"required,max=2000"`
}

// ListNotesRequest meminta semua catatan milik sebuah kontak.
type ListNotesRequest struct {
	UserID    string `json:"-" validate:"required"`
	ContactID string `json:"-" validate:"required,uuid"`
}

// DeleteNoteRequest mengidentifikasi catatan yang akan dihapus.
type DeleteNoteRequest struct {
	UserID    string `json:"-" validate:"required"`
	ContactID string `json:"-" validate:"required,uuid"`
	ID        string `json:"-" validate:"required,uuid"`
}
```

`internal/model/converter/note_converter.go`:

```go
// NoteToResponse mengubah entity catatan menjadi respons API.
func NoteToResponse(note *entity.Note) *model.NoteResponse {
	return &model.NoteResponse{ID: note.ID, Body: note.Body, CreatedAt: note.CreatedAt}
}
```

Perhatikan `json:"-"` pada `UserID` dan `ContactID`. Client tidak bisa
mengisinya lewat body.

## Langkah 4: Port

Tambahkan ke `internal/usecase/ports.go` hanya method yang dibutuhkan use case:

```go
// NoteRepository adalah akses data catatan kontak.
type NoteRepository interface {
	Create(ctx context.Context, note *entity.Note) error
	Delete(ctx context.Context, note *entity.Note) error
	FindByIDAndContactID(ctx context.Context, id, contactID string) (*entity.Note, error)
	FindAllByContactID(ctx context.Context, contactID string) ([]entity.Note, error)
}
```

Lalu generate mock-nya: `make mocks` (atau `mockery`). File
`internal/usecase/mocks/note_repository.go` akan muncul.

## Langkah 5: Repository

`internal/repository/note_repository.go`:

```go
// NoteRepository mengakses tabel contact_notes.
type NoteRepository struct {
	Repository[entity.Note]   // Create dan Delete gratis dari repository generik
}

// NewNoteRepository membuat NoteRepository.
func NewNoteRepository(db *gorm.DB) *NoteRepository {
	return &NoteRepository{Repository: Repository[entity.Note]{DB: db, Name: "note"}}
}

// FindByIDAndContactID mencari catatan milik kontak tertentu.
func (r *NoteRepository) FindByIDAndContactID(ctx context.Context, id, contactID string) (*entity.Note, error) {
	return r.take(r.db(ctx).Where("id = ? AND contact_id = ?", id, contactID))
}

// FindAllByContactID mengembalikan semua catatan sebuah kontak, terbaru dulu.
func (r *NoteRepository) FindAllByContactID(ctx context.Context, contactID string) ([]entity.Note, error) {
	var notes []entity.Note
	err := r.db(ctx).Where("contact_id = ?", contactID).Order("created_at DESC").Find(&notes).Error
	return notes, err
}
```

- `r.db(ctx)` otomatis memakai transaksi bila ada di context.
- `r.take(...)` menerjemahkan "tidak ditemukan" menjadi
  `apperror.NotFound("note")`.
- Soft delete bekerja otomatis: `Delete` mengisi `deleted_at`, dan query
  hanya melihat baris aktif.

## Langkah 6: Use case, test dulu

Tulis test **sebelum** use case-nya. `internal/usecase/note_usecase_test.go`:

```go
func TestNoteCreate_ContactNotOwnedIsNotFound(t *testing.T) {
	contacts := mocks.NewContactRepository(t)
	notes := mocks.NewNoteRepository(t)
	uc := usecase.NewNoteUseCase(discardLogger(), newValidator(), &fakeTransactor{},
		contacts, notes, mocks.NewEventPublisher(t), newAudit())
	contacts.EXPECT().FindByIDAndUserID(mock.Anything, contactID, "intruder").Return(nil, apperror.NotFound("contact"))

	_, err := uc.Create(context.Background(), &model.CreateNoteRequest{UserID: "intruder", ContactID: contactID, Body: "halo"})

	assert.Equal(t, apperror.CodeNotFound, codeOf(err))
	// notes tidak di-EXPECT: catatan tidak boleh dibuat.
}

func TestNoteCreate_EmptyBodyIsValidationError(t *testing.T) {
	uc := usecase.NewNoteUseCase(discardLogger(), newValidator(), &fakeTransactor{},
		mocks.NewContactRepository(t), mocks.NewNoteRepository(t), mocks.NewEventPublisher(t), newAudit())

	_, err := uc.Create(context.Background(), &model.CreateNoteRequest{UserID: "u1", ContactID: contactID})

	assert.Equal(t, []string{"body"}, fieldsOf(err))
}
```

(`contactID`, `codeOf`, `fieldsOf`, `discardLogger`, `newValidator`, dan
`newAudit` sudah ada di test helper package ini.)

Jalankan `go test ./internal/usecase/ -run TestNote`. Test **gagal
dikompilasi** karena `NewNoteUseCase` belum ada. Itulah langkah "red" dalam TDD.

Sekarang `internal/usecase/note_usecase.go`:

```go
// NoteUseCase berisi aturan bisnis catatan kontak. Setiap operasi lebih dulu
// memastikan kontak induknya milik user yang login.
type NoteUseCase struct {
	log      *slog.Logger
	validate *validation.Validator
	tx       Transactor
	contacts ContactRepository
	notes    NoteRepository
	events   EventPublisher
	audit    *AuditRecorder
}

// NewNoteUseCase membuat NoteUseCase.
func NewNoteUseCase(log *slog.Logger, validate *validation.Validator, tx Transactor,
	contacts ContactRepository, notes NoteRepository, events EventPublisher, audit *AuditRecorder) *NoteUseCase {
	return &NoteUseCase{log: log, validate: validate, tx: tx, contacts: contacts, notes: notes, events: events, audit: audit}
}

// Create menambahkan catatan ke kontak milik user yang login.
func (c *NoteUseCase) Create(ctx context.Context, req *model.CreateNoteRequest) (*model.NoteResponse, error) {
	if err := c.validate.Struct(req); err != nil {
		return nil, err
	}
	note := &entity.Note{ID: uuid.NewString(), ContactID: req.ContactID, Body: req.Body}

	err := c.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		// 1. Kepemilikan: kontak milik user lain → NOT_FOUND.
		if _, err := c.contacts.FindByIDAndUserID(ctx, req.ContactID, req.UserID); err != nil {
			return err
		}
		// 2. Simpan.
		if err := c.notes.Create(ctx, note); err != nil {
			return err
		}
		// 3. Audit dan event, di transaksi yang sama.
		res := converter.NoteToResponse(note)
		if err := c.audit.Record(ctx, "note.created", "note", note.ID, nil, res); err != nil {
			return err
		}
		return publish(ctx, c.events, "note.created", note.ID, res)
	})
	if err != nil {
		return nil, err
	}
	return converter.NoteToResponse(note), nil
}

// List mengembalikan semua catatan sebuah kontak milik user yang login.
func (c *NoteUseCase) List(ctx context.Context, req *model.ListNotesRequest) ([]model.NoteResponse, error) {
	if err := c.validate.Struct(req); err != nil {
		return nil, err
	}
	if _, err := c.contacts.FindByIDAndUserID(ctx, req.ContactID, req.UserID); err != nil {
		return nil, err
	}
	notes, err := c.notes.FindAllByContactID(ctx, req.ContactID)
	if err != nil {
		return nil, err
	}
	responses := make([]model.NoteResponse, len(notes))
	for i := range notes {
		responses[i] = *converter.NoteToResponse(&notes[i])
	}
	return responses, nil
}

// Delete menghapus (soft delete) catatan.
func (c *NoteUseCase) Delete(ctx context.Context, req *model.DeleteNoteRequest) error {
	if err := c.validate.Struct(req); err != nil {
		return err
	}
	return c.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		if _, err := c.contacts.FindByIDAndUserID(ctx, req.ContactID, req.UserID); err != nil {
			return err
		}
		note, err := c.notes.FindByIDAndContactID(ctx, req.ID, req.ContactID)
		if err != nil {
			return err
		}
		if err := c.notes.Delete(ctx, note); err != nil {
			return err
		}
		res := converter.NoteToResponse(note)
		if err := c.audit.Record(ctx, "note.deleted", "note", note.ID, res, nil); err != nil {
			return err
		}
		return publish(ctx, c.events, "note.deleted", note.ID, res)
	})
}
```

Jalankan lagi test-nya. Sekarang hijau.

Satu hal kecil lagi: daftarkan topik untuk event `note.*` di
`internal/model/event.go`, supaya event catatan ikut ke topik `contacts`
(tanpa ini event masuk ke topik `events`):

```go
var topics = map[string]string{
	"user":    "users",
	"contact": "contacts",
	"address": "addresses",
	"note":    "contacts",
}
```

## Langkah 7: Controller

`internal/delivery/http/note_controller.go`:

```go
// NoteController menangani endpoint catatan di bawah sebuah kontak.
type NoteController struct {
	notes *usecase.NoteUseCase
}

// NewNoteController membuat NoteController.
func NewNoteController(notes *usecase.NoteUseCase) *NoteController {
	return &NoteController{notes: notes}
}

// Create menangani POST /api/v1/contacts/:contactId/notes.
func (h *NoteController) Create(c fiber.Ctx) error {
	req := new(model.CreateNoteRequest)
	if err := c.Bind().Body(req); err != nil {
		return fiber.ErrBadRequest
	}
	req.UserID = middleware.CurrentAuth(c).UserID
	req.ContactID = c.Params("contactId")
	res, err := h.notes.Create(c.Context(), req)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(model.WebResponse[*model.NoteResponse]{Data: res})
}

// List menangani GET /api/v1/contacts/:contactId/notes.
func (h *NoteController) List(c fiber.Ctx) error {
	req := &model.ListNotesRequest{UserID: middleware.CurrentAuth(c).UserID, ContactID: c.Params("contactId")}
	res, err := h.notes.List(c.Context(), req)
	if err != nil {
		return err
	}
	return c.JSON(model.WebResponse[[]model.NoteResponse]{Data: res})
}

// Delete menangani DELETE /api/v1/contacts/:contactId/notes/:noteId.
func (h *NoteController) Delete(c fiber.Ctx) error {
	req := &model.DeleteNoteRequest{
		UserID:    middleware.CurrentAuth(c).UserID,
		ContactID: c.Params("contactId"),
		ID:        c.Params("noteId"),
	}
	if err := h.notes.Delete(c.Context(), req); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
```

## Langkah 8: Route

Di `internal/delivery/http/route/route.go`, tambahkan field ke `RouteConfig`:

```go
	NoteController    *http.NoteController
```

lalu daftarkan route di `setupAuthRoutes` (grup yang wajib login):

```go
	api.Get("/contacts/:contactId/notes", c.NoteController.List)
	api.Post("/contacts/:contactId/notes", c.NoteController.Create)
	api.Delete("/contacts/:contactId/notes/:noteId", c.NoteController.Delete)
```

## Langkah 9: Wiring

Di `internal/config/web.go`, `BootstrapWeb`:

```go
	noteRepository := repository.NewNoteRepository(deps.DB)
	...
	noteUseCase := usecase.NewNoteUseCase(deps.Log, validate, tx, contactRepository, noteRepository, events, audit)
	...
	routes := route.RouteConfig{
		...
		NoteController: http.NewNoteController(noteUseCase),
	}
```

Jalankan `go build ./...`. Bila ada yang terlewat, compiler langsung
memberi tahu. Ini keuntungan wiring manual.

## Langkah 10: Dokumentasi OpenAPI

Jalankan `go test -tags integration -run TestOpenAPI ./test/`. Test ini
**gagal** dan menyebutkan tiga route baru yang belum didokumentasikan.
Tambahkan ke `api/openapi.yaml` (di bawah `paths`):

```yaml
  /contacts/{contactId}/notes:
    parameters:
      - $ref: "#/components/parameters/ContactId"
    get:
      tags: [Note]
      summary: Daftar catatan kontak
      responses:
        "200":
          description: Daftar catatan
          content:
            application/json:
              schema:
                type: object
                properties:
                  data: { type: array, items: { $ref: "#/components/schemas/Note" } }
        "404": { $ref: "#/components/responses/Problem" }
    post:
      tags: [Note]
      summary: Tambah catatan
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [body]
              properties:
                body: { type: string, maxLength: 2000 }
      responses:
        "201":
          description: Catatan dibuat
          content:
            application/json:
              schema: { type: object, properties: { data: { $ref: "#/components/schemas/Note" } } }
        "400": { $ref: "#/components/responses/Problem" }
        "404": { $ref: "#/components/responses/Problem" }

  /contacts/{contactId}/notes/{noteId}:
    parameters:
      - $ref: "#/components/parameters/ContactId"
      - { name: noteId, in: path, required: true, schema: { type: string, format: uuid } }
    delete:
      tags: [Note]
      summary: Hapus catatan (soft delete)
      responses:
        "204": { description: Catatan dihapus }
        "404": { $ref: "#/components/responses/Problem" }
```

dan skemanya di `components.schemas`:

```yaml
    Note:
      type: object
      properties:
        id: { type: string, format: uuid }
        body: { type: string }
        created_at: { type: integer }
```

## Langkah 11: Integration test

`test/note_test.go`:

```go
//go:build integration

package test

func TestNotes_CreateListDelete(t *testing.T) {
	app, userID, token := loggedIn(t)
	contact := app.CreateContact(t, userID, "Budi")
	path := "/api/v1/contacts/" + contact.ID + "/notes"

	resp, raw := app.Do(t, http.MethodPost, path, model.CreateNoteRequest{Body: "Suka kopi"}, token)
	require.Equal(t, http.StatusCreated, resp.StatusCode, string(raw))
	note := testsupport.DecodeData[model.NoteResponse](t, raw)

	_, raw = app.Do(t, http.MethodGet, path, nil, token)
	assert.Len(t, testsupport.DecodeData[[]model.NoteResponse](t, raw), 1)

	resp, _ = app.Do(t, http.MethodDelete, path+"/"+note.ID, nil, token)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	_, raw = app.Do(t, http.MethodGet, path, nil, token)
	assert.Empty(t, testsupport.DecodeData[[]model.NoteResponse](t, raw))
}

func TestNotes_OtherUsersContactIsNotFound(t *testing.T) {
	app, _, token := loggedIn(t)
	other := app.CreateUser(t, "other@example.com", "rahasia123")
	contact := app.CreateContact(t, other.ID, "Rahasia")

	resp, _ := app.Do(t, http.MethodPost, "/api/v1/contacts/"+contact.ID+"/notes", model.CreateNoteRequest{Body: "x"}, token)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
```

(Bagian `import` dihilangkan supaya ringkas; contek dari `test/contact_test.go`.)

## Langkah 12: Verifikasi akhir

```bash
gofmt -l .                                # harus kosong
go build ./... && go vet ./...
golangci-lint run ./...                   # harus 0 issues (termasuk doc comment)
go test ./internal/...
go test -tags integration -count=1 ./test/...
```

Lalu coba manual lewat Swagger UI di <http://localhost:3000/docs>.

## Apa yang Anda dapat "gratis" dari pola ini

Tanpa menulis kode tambahan, fitur catatan sudah memiliki:

- autentikasi, rate limit, timeout, header keamanan (middleware global),
- format error Problem Details dan validasi per field,
- pengecekan kepemilikan,
- soft delete,
- audit trail (`note.created`, `note.deleted`),
- event di outbox, terkirim ke Kafka atau handler lokal,
- metrik dan trace per route,
- dokumentasi yang dijaga test.

## Tantangan lanjutan

1. Tambahkan `PUT .../notes/{noteId}` dengan **optimistic locking**
   (kolom `version`, `UpdateWithVersion`, respons 409).
2. Tampilkan catatan di `GET /api/v1/contacts/{id}` sebagai field
   `notes` opsional (`?include=notes`).
3. Tambahkan `NoteService` ke gRPC (`api/proto/note/v1/note.proto`), memakai
   `NoteUseCase` yang sama.
4. Buat handler event yang mencatat jumlah catatan per kontak.
