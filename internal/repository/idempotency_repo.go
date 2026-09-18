package repository

import (
	"context"
	"errors"
	"time"

	"task-api/internal/middleware"

	"gorm.io/gorm"
)

const idempotencyTTL = 24 * time.Hour

// idempotencyModel is the persistence model of stored idempotent responses.
type idempotencyModel struct {
	Key       string    `gorm:"primaryKey;column:key"`
	Status    int       `gorm:"not null;default:200"`
	Response  string    `gorm:"type:text"`
	CreatedAt time.Time `gorm:"not null"`
}

func (idempotencyModel) TableName() string { return "idempotency_records" }

// idempotencyStore adapts PostgreSQL to the middleware.IdempotencyStore port.
type idempotencyStore struct {
	db *gorm.DB
}

func NewIdempotencyStore(db *gorm.DB) middleware.IdempotencyStore {
	return &idempotencyStore{db: db}
}

func (s *idempotencyStore) Get(ctx context.Context, key string) (*middleware.IdempotencyRecord, bool, error) {
	var m idempotencyModel
	err := s.db.WithContext(ctx).Where("key = ?", key).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	if time.Since(m.CreatedAt) >= idempotencyTTL {
		// Expired entry: clean it up opportunistically and report a miss.
		_ = s.db.WithContext(ctx).Delete(&idempotencyModel{}, "key = ?", key).Error
		return nil, false, nil
	}

	return &middleware.IdempotencyRecord{
		Key:       m.Key,
		Status:    m.Status,
		Body:      m.Response,
		CreatedAt: m.CreatedAt,
	}, true, nil
}

func (s *idempotencyStore) Save(ctx context.Context, key string, status int, body string) error {
	m := idempotencyModel{Key: key, Status: status, Response: body, CreatedAt: time.Now()}
	return s.db.WithContext(ctx).Create(&m).Error
}
