package repository

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"task-api/internal/domain/task"

	"gorm.io/gorm"
)

// taskModel is the persistence model of the Task aggregate. It is kept
// separate from the domain entity so the domain stays free of GORM concerns.
type taskModel struct {
	ID          string `gorm:"primaryKey;type:uuid"`
	Title       string `gorm:"type:varchar(255);not null"`
	Description string `gorm:"type:text"`
	Status      string `gorm:"type:varchar(20);not null"`
	AssigneeID  string `gorm:"type:uuid;not null;index"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (taskModel) TableName() string { return "tasks" }

// taskLogModel is the persistence model of the TaskLog child entity.
type taskLogModel struct {
	ID        string `gorm:"primaryKey;type:uuid"`
	TaskID    string `gorm:"type:uuid;not null;index"`
	Action    string `gorm:"type:varchar(50);not null"`
	OldValue  string `gorm:"type:text"`
	NewValue  string `gorm:"type:text"`
	ChangedBy string `gorm:"type:uuid;not null"`
	CreatedAt time.Time
}

func (taskLogModel) TableName() string { return "task_logs" }

func taskToDomain(m taskModel) *task.Task {
	return &task.Task{
		ID:          m.ID,
		Title:       m.Title,
		Description: m.Description,
		Status:      task.TaskStatus(m.Status),
		AssigneeID:  m.AssigneeID,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

func taskFromDomain(t *task.Task) taskModel {
	return taskModel{
		ID:          t.ID,
		Title:       t.Title,
		Description: t.Description,
		Status:      t.Status.String(),
		AssigneeID:  t.AssigneeID,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}

type taskRepo struct {
	db *gorm.DB
}

func NewTaskRepository(db *gorm.DB) task.Repository {
	return &taskRepo{db: db}
}

func (r *taskRepo) Create(ctx context.Context, t *task.Task) error {
	m := taskFromDomain(t)
	return r.db.WithContext(ctx).Create(&m).Error
}

func (r *taskRepo) FindAll(ctx context.Context, filter task.Filter) ([]task.Task, int64, error) {
	var models []taskModel
	var count int64

	query := r.db.WithContext(ctx).Model(&taskModel{}).Where("assignee_id = ?", filter.Assignee)

	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.Title != "" {
		query = query.Where("title LIKE ?", "%"+filter.Title+"%")
	}

	if err := query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}

	if err := query.Find(&models).Error; err != nil {
		return nil, 0, err
	}

	tasks := make([]task.Task, 0, len(models))
	for _, m := range models {
		tasks = append(tasks, *taskToDomain(m))
	}
	return tasks, count, nil
}

func (r *taskRepo) FindByID(ctx context.Context, id string) (*task.Task, error) {
	var m taskModel
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Translate the infrastructure error into a domain error so
		// upper layers never depend on GORM.
		return nil, task.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return taskToDomain(m), nil
}

func (r *taskRepo) Update(ctx context.Context, t *task.Task) error {
	m := taskFromDomain(t)
	return r.db.WithContext(ctx).Model(&taskModel{ID: m.ID}).Updates(map[string]any{
		"title":       m.Title,
		"description": m.Description,
		"status":      m.Status,
		"assignee_id": m.AssigneeID,
		"updated_at":  m.UpdatedAt,
	}).Error
}

func (r *taskRepo) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&taskModel{}, "id = ?", id).Error
}

// SaveAssignment persists the aggregate's assignee change and its audit log
// inside a single database transaction.
func (r *taskRepo) SaveAssignment(ctx context.Context, t *task.Task, log *task.TaskLog) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&taskModel{ID: t.ID}).Updates(map[string]any{
			"assignee_id": t.AssigneeID,
			"updated_at":  t.UpdatedAt,
		}).Error; err != nil {
			return err
		}

		lm := taskLogModel{
			ID:        log.ID,
			TaskID:    log.TaskID,
			Action:    log.Action,
			OldValue:  log.OldValue,
			NewValue:  log.NewValue,
			ChangedBy: log.ChangedBy,
			CreatedAt: log.CreatedAt,
		}
		if err := tx.Create(&lm).Error; err != nil {
			return err
		}

		// Mock notification as part of the assignment flow. In production this
		// would be an outbox/event publish; here a structured log is enough.
		slog.Default().Info("assignment notification sent",
			slog.String("task_id", t.ID),
			slog.String("new_assignee_id", t.AssigneeID),
			slog.String("changed_by", log.ChangedBy),
		)
		return nil
	})
}
