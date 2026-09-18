package task

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	maxTitleLength = 255
	actionAssigned = "assigned"
)

// Task is the aggregate root of the Task aggregate.
// It owns its identity, its invariants, and its audit trail (TaskLog).
type Task struct {
	ID          string
	Title       string
	Description string
	Status      TaskStatus
	ProjectID   string
	AssigneeID  string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// TaskLog is a child entity of the Task aggregate: an audit record of
// an assignment change. It has no meaning outside its aggregate.
type TaskLog struct {
	ID        string
	TaskID    string
	Action    string
	OldValue  string
	NewValue  string
	ChangedBy string
	CreatedAt time.Time
}

// NewTask is the factory of the aggregate. Business rules: every new task
// starts in "pending", always has an owner, and always lives in a project.
func NewTask(title, description, assigneeID, projectID string) (*Task, error) {
	title = strings.TrimSpace(title)
	if title == "" || len(title) > maxTitleLength {
		return nil, ErrInvalidTitle
	}
	if assigneeID == "" {
		return nil, ErrInvalidAssignee
	}
	if projectID == "" {
		return nil, ErrInvalidProject
	}

	now := time.Now()
	return &Task{
		ID:          uuid.NewString(),
		Title:       title,
		Description: description,
		Status:      StatusPending,
		ProjectID:   projectID,
		AssigneeID:  assigneeID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// EnsureOwnedBy enforces the ownership invariant of the aggregate.
func (t *Task) EnsureOwnedBy(userID string) error {
	if t.AssigneeID != userID {
		return ErrForbidden
	}
	return nil
}

// ChangeDetails applies a partial update. A nil pointer means
// "leave this field unchanged".
func (t *Task) ChangeDetails(title, description *string, status *TaskStatus) error {
	if title != nil {
		trimmed := strings.TrimSpace(*title)
		if trimmed == "" || len(trimmed) > maxTitleLength {
			return ErrInvalidTitle
		}
		t.Title = trimmed
	}
	if description != nil {
		t.Description = *description
	}
	if status != nil {
		t.Status = *status
	}
	t.UpdatedAt = time.Now()
	return nil
}

// AssignTo reassigns the task. Invariants owned by the aggregate:
//   - a new assignee id is required
//   - every reassignment must produce an audit log (TaskLog)
//
// Authorization (the changer must be a member of the task's project) is
// enforced by the application service, which owns the membership data.
func (t *Task) AssignTo(newAssigneeID, changedBy string) (*TaskLog, error) {
	if newAssigneeID == "" {
		return nil, ErrInvalidAssignee
	}

	oldValue := t.AssigneeID
	t.AssigneeID = newAssigneeID
	t.UpdatedAt = time.Now()

	return &TaskLog{
		ID:        uuid.NewString(),
		TaskID:    t.ID,
		Action:    actionAssigned,
		OldValue:  oldValue,
		NewValue:  newAssigneeID,
		ChangedBy: changedBy,
		CreatedAt: time.Now(),
	}, nil
}
