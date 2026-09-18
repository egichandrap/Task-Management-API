package repository

import (
	"context"
	"task-api/internal/domain"
	"gorm.io/gorm"
)

type taskRepo struct {
	db *gorm.DB
}

func NewTaskRepository(db *gorm.DB) domain.TaskRepository {
	return &taskRepo{db: db}
}

func (r *taskRepo) Create(ctx context.Context, task *domain.Task) error {
	return r.db.WithContext(ctx).Create(task).Error
}

func (r *taskRepo) FindAll(ctx context.Context, filter domain.TaskFilter) ([]domain.Task, int64, error) {
	var tasks []domain.Task
	var count int64
	
	query := r.db.WithContext(ctx).Model(&domain.Task{}).Where("assignee_id = ?", filter.Assignee)
	
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.Title != "" {
		query = query.Where("title LIKE ?", "%"+filter.Title+"%")
	}
	
	query.Count(&count)
	
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}
	
	err := query.Find(&tasks).Error
	return tasks, count, err
}

func (r *taskRepo) FindByID(ctx context.Context, id string) (*domain.Task, error) {
	var task domain.Task
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&task).Error
	return &task, err
}

func (r *taskRepo) Update(ctx context.Context, task *domain.Task) error {
	return r.db.WithContext(ctx).Save(task).Error
}

func (r *taskRepo) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&domain.Task{}, "id = ?", id).Error
}

func (r *taskRepo) AssignTaskTx(ctx context.Context, taskID, newAssigneeID, changedBy string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var task domain.Task
		if err := tx.Where("id = ?", taskID).First(&task).Error; err != nil {
			return err
		}
		
		oldAssignee := task.AssigneeID
		task.AssigneeID = newAssigneeID
		
		if err := tx.Save(&task).Error; err != nil {
			return err
		}
		
		log := domain.TaskLog{
			TaskID:    taskID,
			Action:    "assigned",
			OldValue:  oldAssignee,
			NewValue:  newAssigneeID,
			ChangedBy: changedBy,
		}
		if err := tx.Create(&log).Error; err != nil {
			return err
		}
		
		// Mock notification
		// log.Println("Notification sent for task assignment")
		
		return nil
	})
}

func (r *taskRepo) SaveIdempotency(ctx context.Context, key string, response string) error {
	record := domain.IdempotencyRecord{
		Key:      key,
		Response: response,
	}
	return r.db.WithContext(ctx).Create(&record).Error
}

func (r *taskRepo) GetIdempotency(ctx context.Context, key string) (*domain.IdempotencyRecord, error) {
	var record domain.IdempotencyRecord
	err := r.db.WithContext(ctx).Where("key = ?", key).First(&record).Error
	return &record, err
}
