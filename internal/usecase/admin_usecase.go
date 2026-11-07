package usecase

import (
	"context"
	"log/slog"

	"github.com/primestrider/golang-clean-architecture/internal/apperror"
	"github.com/primestrider/golang-clean-architecture/internal/entity"
	"github.com/primestrider/golang-clean-architecture/internal/model"
	"github.com/primestrider/golang-clean-architecture/internal/model/converter"
	"github.com/primestrider/golang-clean-architecture/internal/validation"
)

// AdminUseCase berisi operasi khusus admin. Pemeriksaan role dilakukan di
// delivery (middleware RequireRole); use case ini fokus pada aturan bisnis
// seperti "admin tidak boleh menurunkan role dirinya sendiri".
type AdminUseCase struct {
	log      *slog.Logger
	validate *validation.Validator
	tx       Transactor
	users    UserRepository
	events   EventPublisher
	audit    *AuditRecorder
}

// NewAdminUseCase membuat AdminUseCase.
func NewAdminUseCase(log *slog.Logger, validate *validation.Validator, tx Transactor,
	users UserRepository, events EventPublisher, audit *AuditRecorder) *AdminUseCase {
	return &AdminUseCase{log: log, validate: validate, tx: tx, users: users, events: events, audit: audit}
}

// ListUsers mengembalikan satu halaman daftar user.
func (c *AdminUseCase) ListUsers(ctx context.Context, req *model.ListUsersRequest) ([]model.UserResponse, *model.PageMetadata, error) {
	if err := c.validate.Struct(req); err != nil {
		return nil, nil, err
	}
	users, total, err := c.users.List(ctx, req.Page, req.Size)
	if err != nil {
		return nil, nil, err
	}
	responses := make([]model.UserResponse, len(users))
	for i := range users {
		responses[i] = *converter.UserToResponse(&users[i])
	}
	return responses, model.NewPageMetadata(req.Page, req.Size, total), nil
}

// ChangeRole mengubah role user. Admin tidak boleh mengubah role dirinya
// sendiri, supaya sistem tidak bisa kehilangan admin terakhir karena salah klik.
//
// Role baru berlaku pada access token berikutnya (saat login atau refresh),
// karena role tersimpan di dalam token yang sudah terbit.
func (c *AdminUseCase) ChangeRole(ctx context.Context, req *model.ChangeRoleRequest) (*model.UserResponse, error) {
	if err := c.validate.Struct(req); err != nil {
		return nil, err
	}
	if req.ActorID == req.UserID {
		return nil, apperror.Conflict("admins cannot change their own role")
	}

	var user *entity.User
	err := c.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		var err error
		user, err = c.users.FindByID(ctx, req.UserID)
		if err != nil {
			return err
		}
		before := converter.UserToResponse(user)
		user.Role = req.Role
		if err := c.users.UpdateWithVersion(ctx, user, req.Version); err != nil {
			return err
		}
		if err := c.audit.Record(ctx, "user.role_changed", "user", user.ID, before, converter.UserToResponse(user)); err != nil {
			return err
		}
		return publish(ctx, c.events, "user.role_changed", user.ID, converter.UserToResponse(user))
	})
	if err != nil {
		return nil, err
	}
	return converter.UserToResponse(user), nil
}

// PromoteByEmail menjadikan user dengan email tertentu sebagai admin.
// Dipakai CLI create-admin untuk membuat admin pertama, ketika belum ada
// admin yang bisa memakai endpoint ChangeRole.
func (c *AdminUseCase) PromoteByEmail(ctx context.Context, email string) (*model.UserResponse, error) {
	var user *entity.User
	err := c.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		var err error
		user, err = c.users.FindByEmail(ctx, NormalizeEmail(email))
		if err != nil {
			return err
		}
		before := converter.UserToResponse(user)
		user.Role = entity.RoleAdmin
		if err := c.users.Update(ctx, user); err != nil {
			return err
		}
		if err := c.audit.Record(ctx, "user.role_changed", "user", user.ID, before, converter.UserToResponse(user)); err != nil {
			return err
		}
		return publish(ctx, c.events, "user.role_changed", user.ID, converter.UserToResponse(user))
	})
	if err != nil {
		return nil, err
	}
	return converter.UserToResponse(user), nil
}
