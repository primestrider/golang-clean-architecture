package repository

import (
	"golang-clean-architecture/internal/entity"
	"log/slog"

	"gorm.io/gorm"
)

type UserRepository struct {
	Repository[entity.User]
	Log *slog.Logger
}

func NewUserRepository(log *slog.Logger) *UserRepository {
	return &UserRepository{
		Log: log,
	}
}

func (r *UserRepository) FindByToken(db *gorm.DB, user *entity.User, token string) error {
	return db.Where("token = ?", token).First(user).Error
}
