package usecase

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/primestrider/golang-clean-architecture/internal/apperror"
	"github.com/primestrider/golang-clean-architecture/internal/entity"
	"github.com/primestrider/golang-clean-architecture/internal/model"
	"github.com/primestrider/golang-clean-architecture/internal/model/converter"
	"github.com/primestrider/golang-clean-architecture/internal/validation"
	"golang.org/x/crypto/bcrypt"
)

// UserUseCase berisi aturan bisnis untuk akun user: registrasi, login,
// profil, dan logout.
type UserUseCase struct {
	log      *slog.Logger
	validate *validation.Validator
	tx       Transactor
	users    UserRepository
	events   EventPublisher
}

// NewUserUseCase membuat UserUseCase dengan dependency-nya.
func NewUserUseCase(log *slog.Logger, validate *validation.Validator, tx Transactor,
	users UserRepository, events EventPublisher) *UserUseCase {
	return &UserUseCase{log: log, validate: validate, tx: tx, users: users, events: events}
}

// Verify mencari user pemilik token dan mengembalikan identitasnya.
// Dipakai middleware autentikasi di setiap request yang butuh login.
func (c *UserUseCase) Verify(ctx context.Context, req *model.VerifyUserRequest) (*model.Auth, error) {
	if err := c.validate.Struct(req); err != nil {
		return nil, apperror.Unauthorized("invalid token")
	}
	user, err := c.users.FindByToken(ctx, req.Token)
	if err != nil {
		if isNotFound(err) {
			return nil, apperror.Unauthorized("invalid token")
		}
		return nil, err
	}
	return &model.Auth{ID: user.ID}, nil
}

// Create mendaftarkan user baru. Password disimpan sebagai hash bcrypt.
func (c *UserUseCase) Create(ctx context.Context, req *model.RegisterUserRequest) (*model.UserResponse, error) {
	if err := c.validate.Struct(req); err != nil {
		return nil, err
	}

	password, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	user := &entity.User{ID: req.ID, Password: string(password), Name: req.Name}

	err = c.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		_, err := c.users.FindByID(ctx, req.ID)
		switch {
		case err == nil:
			return apperror.Conflict("user already exists")
		case !isNotFound(err):
			return err
		}
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

// Login memeriksa kredensial dan menerbitkan token sesi baru. Pesan error
// sengaja sama untuk user tidak ditemukan dan password salah, supaya
// penyerang tidak bisa menebak user mana yang terdaftar.
func (c *UserUseCase) Login(ctx context.Context, req *model.LoginUserRequest) (*model.UserResponse, error) {
	if err := c.validate.Struct(req); err != nil {
		return nil, err
	}

	var user *entity.User
	err := c.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		var err error
		user, err = c.users.FindByID(ctx, req.ID)
		if err != nil {
			if isNotFound(err) {
				return apperror.Unauthorized("invalid credentials")
			}
			return err
		}
		if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)) != nil {
			return apperror.Unauthorized("invalid credentials")
		}
		user.Token = uuid.NewString()
		return c.users.Update(ctx, user)
	})
	if err != nil {
		return nil, err
	}
	return converter.UserToTokenResponse(user), nil
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

// Logout menghapus token sesi user sehingga token lama tidak berlaku lagi.
func (c *UserUseCase) Logout(ctx context.Context, req *model.LogoutUserRequest) (bool, error) {
	if err := c.validate.Struct(req); err != nil {
		return false, err
	}
	err := c.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		user, err := c.users.FindByID(ctx, req.ID)
		if err != nil {
			return err
		}
		user.Token = ""
		return c.users.Update(ctx, user)
	})
	return err == nil, err
}

// Update mengubah nama dan/atau password user yang login.
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
		if req.Password != "" {
			hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
			if err != nil {
				return apperror.Internal(err)
			}
			user.Password = string(hash)
		}
		if err := c.users.Update(ctx, user); err != nil {
			return err
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
