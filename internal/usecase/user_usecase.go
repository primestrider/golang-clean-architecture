package usecase

import (
	"context"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/primestrider/golang-clean-architecture/internal/apperror"
	"github.com/primestrider/golang-clean-architecture/internal/entity"
	"github.com/primestrider/golang-clean-architecture/internal/model"
	"github.com/primestrider/golang-clean-architecture/internal/model/converter"
	"github.com/primestrider/golang-clean-architecture/internal/security"
	"github.com/primestrider/golang-clean-architecture/internal/validation"
)

// UserUseCase berisi aturan bisnis akun user: registrasi dan profil.
// Login, refresh token, dan logout ada di AuthUseCase.
type UserUseCase struct {
	log      *slog.Logger
	validate *validation.Validator
	tx       Transactor
	users    UserRepository
	tokens   RefreshTokenRepository
	events   EventPublisher
}

// NewUserUseCase membuat UserUseCase dengan dependency-nya.
func NewUserUseCase(log *slog.Logger, validate *validation.Validator, tx Transactor,
	users UserRepository, tokens RefreshTokenRepository, events EventPublisher) *UserUseCase {
	return &UserUseCase{log: log, validate: validate, tx: tx, users: users, tokens: tokens, events: events}
}

// NormalizeEmail merapikan email (huruf kecil, tanpa spasi di tepi) supaya
// "Eko@Example.com" dan "eko@example.com" dianggap email yang sama.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// Create mendaftarkan user baru dengan role "user". Password disimpan
// sebagai hash bcrypt.
func (c *UserUseCase) Create(ctx context.Context, req *model.RegisterUserRequest) (*model.UserResponse, error) {
	req.Email = NormalizeEmail(req.Email)
	if err := c.validate.Struct(req); err != nil {
		return nil, err
	}

	hash, err := security.HashPassword(req.Password)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	user := &entity.User{
		ID:       uuid.NewString(),
		Email:    req.Email,
		Name:     req.Name,
		Password: hash,
		Role:     entity.RoleUser,
		Version:  1,
	}

	err = c.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := c.users.Create(ctx, user); err != nil {
			return err
		}
		return c.events.Publish(ctx, converter.UserToEvent(user))
	})
	if err != nil {
		return nil, err
	}
	return converter.UserToResponse(user), nil
}

// Current mengembalikan profil user yang login.
func (c *UserUseCase) Current(ctx context.Context, req *model.GetUserRequest) (*model.UserResponse, error) {
	if err := c.validate.Struct(req); err != nil {
		return nil, err
	}
	user, err := c.users.FindByID(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	return converter.UserToResponse(user), nil
}

// Update mengubah nama dan/atau password user yang login.
//
// Mengganti password wajib membuktikan password lama, supaya orang yang
// sempat memegang sesi (misalnya laptop yang ditinggal) tidak bisa
// mengambil alih akun. Setelah password diganti, semua refresh token user
// dicabut sehingga sesi di perangkat lain harus login ulang.
func (c *UserUseCase) Update(ctx context.Context, req *model.UpdateUserRequest) (*model.UserResponse, error) {
	if err := c.validate.Struct(req); err != nil {
		return nil, err
	}

	var user *entity.User
	err := c.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		var err error
		user, err = c.users.FindByID(ctx, req.ID)
		if err != nil {
			return err
		}
		if req.Name != "" {
			user.Name = req.Name
		}

		passwordChanged := false
		if req.NewPassword != "" {
			if req.CurrentPassword == "" || !security.ComparePassword(user.Password, req.CurrentPassword) {
				return apperror.Validation([]apperror.FieldError{{Field: "current_password", Message: "is incorrect"}})
			}
			hash, err := security.HashPassword(req.NewPassword)
			if err != nil {
				return apperror.Internal(err)
			}
			user.Password = hash
			passwordChanged = true
		}

		if err := c.users.Update(ctx, user); err != nil {
			return err
		}
		if passwordChanged {
			if err := c.tokens.RevokeAllForUser(ctx, user.ID); err != nil {
				return err
			}
		}
		return c.events.Publish(ctx, converter.UserToEvent(user))
	})
	if err != nil {
		return nil, err
	}
	return converter.UserToResponse(user), nil
}

// isNotFound melaporkan apakah err adalah apperror NOT_FOUND.
func isNotFound(err error) bool {
	appErr, ok := apperror.As(err)
	return ok && appErr.Code == apperror.CodeNotFound
}
