package usecase

import (
	"context"
	"encoding/json"
	"maps"
	"reflect"

	"github.com/google/uuid"
	"github.com/primestrider/golang-clean-architecture/internal/apperror"
	"github.com/primestrider/golang-clean-architecture/internal/entity"
	"github.com/primestrider/golang-clean-architecture/internal/observability"
)

// alwaysExcluded adalah field yang tidak pernah boleh masuk jejak audit:
// data rahasia, dan timestamp yang selalu berubah sehingga hanya menjadi noise.
var alwaysExcluded = []string{"password", "token_hash", "refresh_token", "access_token", "created_at", "updated_at"}

// AuditRecorder menulis jejak audit: siapa melakukan apa pada data mana.
//
// Record dipanggil di dalam transaksi bisnis. Dengan begitu catatan audit
// hanya tersimpan bila perubahan datanya juga tersimpan, dan sebaliknya.
type AuditRecorder struct {
	repo AuditRepository
}

// NewAuditRecorder membuat AuditRecorder.
func NewAuditRecorder(repo AuditRepository) *AuditRecorder {
	return &AuditRecorder{repo: repo}
}

// Record mencatat satu aksi. Actor dan request id diambil dari context
// (diisi middleware). before dan after adalah keadaan data sebelum dan
// sesudah aksi (boleh nil untuk create atau delete); yang disimpan hanya
// field yang berubah.
func (a *AuditRecorder) Record(ctx context.Context, action, entityType, entityID string, before, after any) error {
	var changes []byte
	if diff := diffChanges(before, after); len(diff) > 0 {
		var err error
		if changes, err = json.Marshal(diff); err != nil {
			return apperror.Internal(err)
		}
	}
	return a.repo.Create(ctx, &entity.AuditLog{
		ID:         uuid.NewString(),
		ActorID:    optional(observability.UserID(ctx)),
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		RequestID:  optional(observability.RequestID(ctx)),
		Changes:    changes,
	})
}

// diffChanges membandingkan dua nilai berdasarkan nama field di tag json dan
// mengembalikan field yang berbeda sebagai pasangan [sebelum, sesudah].
// Field sensitif selalu diabaikan, ditambah field di exclude.
func diffChanges(before, after any, exclude ...string) map[string][2]any {
	b, a := toFieldMap(before), toFieldMap(after)
	skip := map[string]bool{}
	for _, f := range append(alwaysExcluded, exclude...) {
		skip[f] = true
	}

	keys := maps.Clone(b)
	maps.Copy(keys, a)
	changes := map[string][2]any{}
	for key := range keys {
		if skip[key] {
			continue
		}
		if !reflect.DeepEqual(b[key], a[key]) {
			changes[key] = [2]any{b[key], a[key]}
		}
	}
	return changes
}

// toFieldMap mengubah struct menjadi map nama-field-json -> nilai, lewat
// encoding JSON supaya nama field mengikuti tag json.
func toFieldMap(v any) map[string]any {
	out := map[string]any{}
	if v == nil || (reflect.ValueOf(v).Kind() == reflect.Pointer && reflect.ValueOf(v).IsNil()) {
		return out
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return out
	}
	_ = json.Unmarshal(raw, &out)
	return out
}

// optional mengubah string kosong menjadi nil untuk kolom nullable.
func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
