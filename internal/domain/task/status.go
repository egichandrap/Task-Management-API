package task

// TaskStatus is a Value Object: an immutable, self-validating task state.
// An invalid TaskStatus cannot exist, so the rest of the code never has
// to re-check it.
type TaskStatus string

const (
	StatusPending    TaskStatus = "pending"
	StatusInProgress TaskStatus = "in-progress"
	StatusCompleted  TaskStatus = "completed"
)

// NewTaskStatus is the only way to create a TaskStatus from raw input.
func NewTaskStatus(s string) (TaskStatus, error) {
	switch ts := TaskStatus(s); ts {
	case StatusPending, StatusInProgress, StatusCompleted:
		return ts, nil
	default:
		return "", ErrInvalidStatus
	}
}

func (s TaskStatus) String() string {
	return string(s)
}
