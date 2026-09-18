package repository

import (
	"context"
	"errors"
	"time"

	"task-api/internal/domain/user"

	"gorm.io/gorm"
)

// userModel is the persistence model of the identity context, kept separate
// from the domain entity so the domain carries no GORM concerns.
type userModel struct {
	ID           string `gorm:"primaryKey;type:uuid"`
	Username     string `gorm:"column:username;type:varchar(255);uniqueIndex;not null"`
	PasswordHash string `gorm:"column:password;type:varchar(255);not null"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (userModel) TableName() string { return "users" }

func userToDomain(m userModel) *user.User {
	return &user.User{
		ID:           m.ID,
		Username:     user.Username(m.Username),
		PasswordHash: m.PasswordHash,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
}

type userRepo struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) user.Repository {
	return &userRepo{db: db}
}

func (r *userRepo) Create(ctx context.Context, u *user.User) error {
	m := userModel{
		ID:           u.ID,
		Username:     u.Username.String(),
		PasswordHash: u.PasswordHash,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	}
	err := r.db.WithContext(ctx).Create(&m).Error
	// Requires gorm.Config{TranslateError: true}; translates the unique
	// constraint violation into a domain error.
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return user.ErrUsernameTaken
	}
	return err
}

func (r *userRepo) GetByUsername(ctx context.Context, username user.Username) (*user.User, error) {
	var m userModel
	err := r.db.WithContext(ctx).Where("username = ?", username.String()).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, user.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return userToDomain(m), nil
}
