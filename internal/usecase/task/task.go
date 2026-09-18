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

// UserChecker is the narrow port this service needs from the User context:
// verifying that a target assignee exists.
type UserChecker interface {
	Exists(ctx context.Context, id string) (bool, error)
}

// MembershipChecker is the narrow port this service needs from the Project
// context: verifying membership of a project.
type MembershipChecker interface {
	IsMember(ctx context.Context, projectID, userID string) (bool, error)
}

type taskUsecase struct {
	repo     task.Repository
	users    UserChecker
	projects MembershipChecker
	log      *slog.Logger
}

// New builds the task application service. The driving port it satisfies is
// defined by the transport layer (consumer side), following the Go idiom of
// declaring interfaces where they are used.
func New(repo task.Repository, users UserChecker, projects MembershipChecker, log *slog.Logger) *taskUsecase {
	return &taskUsecase{repo: repo, users: users, projects: projects, log: log}
}

func (u *taskUsecase) Create(ctx context.Context, title, description, assigneeID, projectID string) (*task.Task, error) {
	// The aggregate factory owns the "new task" rules (valid title, starts pending).
	t, err := task.NewTask(title, description, assigneeID, projectID)
	if err != nil {
		return nil, customerrors.ErrBadRequest
	}

	// The creator must be a member of the project the task lives in.
	// (The initial assignee is always the creator per the API contract.)
	isMember, err := u.projects.IsMember(ctx, projectID, assigneeID)
	if err != nil {
		u.log.Error("check membership failed", slog.Any("error", err), slog.String("project_id", projectID))
		return nil, customerrors.ErrInternalServer
	}
	if !isMember {
		return nil, customerrors.ErrForbidden
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
	// 404 — the task must exist (addressed before authorization).
	t, err := u.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, task.ErrNotFound) {
			return customerrors.ErrNotFound
		}
		u.log.Error("find task failed", slog.Any("error", err), slog.String("task_id", id))
		return customerrors.ErrInternalServer
	}

	// 403 — any member of the task's project may (re)assign it.
	isMember, err := u.projects.IsMember(ctx, t.ProjectID, changedBy)
	if err != nil {
		u.log.Error("check membership failed", slog.Any("error", err), slog.String("project_id", t.ProjectID))
		return customerrors.ErrInternalServer
	}
	if !isMember {
		return customerrors.ErrForbidden
	}

	// The target must be an existing user; otherwise the write would only
	// fail later on the FK constraint and surface as a 500.
	exists, err := u.users.Exists(ctx, newAssigneeID)
	if err != nil {
		u.log.Error("check assignee failed", slog.Any("error", err), slog.String("assignee_id", newAssigneeID))
		return customerrors.ErrInternalServer
	}
	if !exists {
		return customerrors.ErrBadRequest
	}

	// The target must be a member of the same project ("same team").
	targetMember, err := u.projects.IsMember(ctx, t.ProjectID, newAssigneeID)
	if err != nil {
		u.log.Error("check membership failed", slog.Any("error", err), slog.String("project_id", t.ProjectID))
		return customerrors.ErrInternalServer
	}
	if !targetMember {
		return customerrors.ErrBadRequest
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
