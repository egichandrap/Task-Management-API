// Package task contains the application services of the Task bounded
// context. It orchestrates the Task aggregate and its repository port.
package task

import (
	"context"
	"errors"
	"log/slog"

	"task-api/internal/domain/task"
	"task-api/pkg/errors"
)

type taskUsecase struct {
	repo task.Repository
	log  *slog.Logger
}

// New builds the task application service. The driving port it satisfies is
// defined by the transport layer (consumer side), following the Go idiom of
// declaring interfaces where they are used.
func New(repo task.Repository, log *slog.Logger) *taskUsecase {
	return &taskUsecase{repo: repo, log: log}
}

func (u *taskUsecase) Create(ctx context.Context, title, description, assigneeID string) (*task.Task, error) {
	// The aggregate factory owns the "new task" rules (valid title, starts pending).
	t, err := task.NewTask(title, description, assigneeID)
	if err != nil {
		return nil, customerrors.ErrBadRequest
	}

	if err := u.repo.Create(ctx, t); err != nil {
		u.log.Error("create task failed", slog.Any("error", err), slog.String("assignee_id", assigneeID))
		return nil, customerrors.ErrInternalServer
	}

	return t, nil
}

func (u *taskUsecase) List(ctx context.Context, filter task.Filter) ([]task.Task, int64, error) {
	tasks, total, err := u.repo.FindAll(ctx, filter)
	if err != nil {
		u.log.Error("list tasks failed", slog.Any("error", err))
		return nil, 0, customerrors.ErrInternalServer
	}
	return tasks, total, nil
}

// findOwned loads the aggregate and enforces ownership. Ownership violations
// are reported as "not found" so the API never leaks other users' tasks.
func (u *taskUsecase) findOwned(ctx context.Context, id, userID string) (*task.Task, error) {
	t, err := u.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, task.ErrNotFound) {
			return nil, customerrors.ErrNotFound
		}
		u.log.Error("find task failed", slog.Any("error", err), slog.String("task_id", id))
		return nil, customerrors.ErrInternalServer
	}

	if err := t.EnsureOwnedBy(userID); err != nil {
		return nil, customerrors.ErrNotFound
	}
	return t, nil
}

func (u *taskUsecase) Detail(ctx context.Context, id, userID string) (*task.Task, error) {
	return u.findOwned(ctx, id, userID)
}

func (u *taskUsecase) Update(ctx context.Context, id, userID string, title, description, status string) (*task.Task, error) {
	t, err := u.findOwned(ctx, id, userID)
	if err != nil {
		return nil, err
	}

	// Empty string means "not provided" (partial update semantics).
	var titlePtr, descriptionPtr *string
	if title != "" {
		titlePtr = &title
	}
	if description != "" {
		descriptionPtr = &description
	}
	var statusPtr *task.TaskStatus
	if status != "" {
		s, err := task.NewTaskStatus(status)
		if err != nil {
			return nil, customerrors.ErrBadRequest
		}
		statusPtr = &s
	}

	// The aggregate validates and applies the change to its own state.
	if err := t.ChangeDetails(titlePtr, descriptionPtr, statusPtr); err != nil {
		return nil, customerrors.ErrBadRequest
	}

	if err := u.repo.Update(ctx, t); err != nil {
		u.log.Error("update task failed", slog.Any("error", err), slog.String("task_id", id))
		return nil, customerrors.ErrInternalServer
	}

	return t, nil
}

func (u *taskUsecase) Delete(ctx context.Context, id, userID string) error {
	t, err := u.findOwned(ctx, id, userID)
	if err != nil {
		return err
	}

	if err := u.repo.Delete(ctx, t.ID); err != nil {
		u.log.Error("delete task failed", slog.Any("error", err), slog.String("task_id", id))
		return customerrors.ErrInternalServer
	}

	return nil
}

func (u *taskUsecase) Assign(ctx context.Context, id, newAssigneeID, changedBy string) error {
	t, err := u.findOwned(ctx, id, changedBy)
	if err != nil {
		return err
	}

	// The aggregate owns the reassignment rule and produces the audit log.
	logEntry, err := t.AssignTo(newAssigneeID, changedBy)
	if err != nil {
		if errors.Is(err, task.ErrInvalidAssignee) {
			return customerrors.ErrBadRequest
		}
		return customerrors.ErrNotFound
	}

	if err := u.repo.SaveAssignment(ctx, t, logEntry); err != nil {
		u.log.Error("save assignment failed", slog.Any("error", err), slog.String("task_id", id))
		return customerrors.ErrInternalServer
	}

	return nil
}
