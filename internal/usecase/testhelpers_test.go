package usecase_test

import (
	"context"
	"log/slog"

	"github.com/primestrider/golang-clean-architecture/internal/apperror"
	"github.com/primestrider/golang-clean-architecture/internal/entity"
	"github.com/primestrider/golang-clean-architecture/internal/usecase"
	"github.com/primestrider/golang-clean-architecture/internal/validation"
)

// fakeTransactor menjalankan fn langsung tanpa database. Cukup untuk unit
// test, karena yang diuji adalah aturan bisnis, bukan perilaku transaksi
// (perilaku transaksi diuji di integration test).
type fakeTransactor struct {
	// inTx bernilai true selama fn berjalan, supaya test bisa memastikan
	// suatu pemanggilan terjadi di dalam transaksi.
	inTx bool
}

func (f *fakeTransactor) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	f.inTx = true
	defer func() { f.inTx = false }()
	return fn(ctx)
}

// fakeAuditRepo menerima catatan audit tanpa menyimpannya.
type fakeAuditRepo struct{ actions []string }

func (f *fakeAuditRepo) Create(_ context.Context, log *entity.AuditLog) error {
	f.actions = append(f.actions, log.Action)
	return nil
}

func newAudit() *usecase.AuditRecorder { return usecase.NewAuditRecorder(&fakeAuditRepo{}) }

func discardLogger() *slog.Logger { return slog.New(slog.DiscardHandler) }

func newValidator() *validation.Validator { return validation.New() }

// codeOf mengambil apperror.Code dari err, atau string kosong.
func codeOf(err error) apperror.Code {
	if appErr, ok := apperror.As(err); ok {
		return appErr.Code
	}
	return ""
}
