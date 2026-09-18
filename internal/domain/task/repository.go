package task

import "context"

// Filter carries task list query parameters.
type Filter struct {
	Status   string
	Title    string
	Limit    int
	Offset   int
	Assignee string
}

// Repository is the persistence port of the Task aggregate.
// The domain defines the contract; infrastructure implements it.
type Repository interface {
	Create(ctx context.Context, t *Task) error
	FindAll(ctx context.Context, filter Filter) ([]Task, int64, error)
	FindByID(ctx context.Context, id string) (*Task, error)
	Update(ctx context.Context, t *Task) error
	Delete(ctx context.Context, id string) error
	// SaveAssignment persists an assignee change and its audit log atomically.
	SaveAssignment(ctx context.Context, t *Task, log *TaskLog) error
}
