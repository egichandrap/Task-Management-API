package domain

import (
	"context"
	"time"
)

type Task struct {
	ID          string    `json:"id" gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	Title       string    `json:"title" gorm:"type:varchar(255);not null"`
	Description string    `json:"description" gorm:"type:text"`
	Status      string    `json:"status" gorm:"type:varchar(20);not null;default:'pending'"` // pending, in-progress, completed
	AssigneeID  string    `json:"assignee_id" gorm:"type:uuid;not null;index"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type TaskLog struct {
	ID        string    `json:"id" gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	TaskID    string    `json:"task_id" gorm:"type:uuid;not null;index"`
	Action    string    `json:"action" gorm:"type:varchar(50);not null"` // e.g., 'assigned'
	OldValue  string    `json:"old_value" gorm:"type:text"`
	NewValue  string    `json:"new_value" gorm:"type:text"`
	ChangedBy string    `json:"changed_by" gorm:"type:uuid;not null"`
	CreatedAt time.Time `json:"created_at"`
}

type IdempotencyRecord struct {
	Key       string    `gorm:"primaryKey"`
	Response  string    `gorm:"type:text"`
	CreatedAt time.Time `gorm:"not null"`
}

type TaskFilter struct {
	Status   string
	Title    string
	Limit    int
	Offset   int
	Assignee string
}

type TaskRepository interface {
	Create(ctx context.Context, task *Task) error
	FindAll(ctx context.Context, filter TaskFilter) ([]Task, int64, error)
	FindByID(ctx context.Context, id string) (*Task, error)
	Update(ctx context.Context, task *Task) error
	Delete(ctx context.Context, id string) error
	AssignTaskTx(ctx context.Context, taskID, newAssigneeID, changedBy string) error
	
	// Idempotency support
	SaveIdempotency(ctx context.Context, key string, response string) error
	GetIdempotency(ctx context.Context, key string) (*IdempotencyRecord, error)
}
