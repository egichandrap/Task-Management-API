package task_test

import (
	"errors"
	"testing"

	"task-api/internal/domain/task"
)

func TestNewTaskStatus(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    task.TaskStatus
		isValid bool
	}{
		{"valid pending", "pending", task.StatusPending, true},
		{"valid in-progress", "in-progress", task.StatusInProgress, true},
		{"valid completed", "completed", task.StatusCompleted, true},
		{"invalid random", "hacked", "", false},
		{"invalid empty", "", "", false},
		{"invalid uppercase", "PENDING", "", false},
		{"invalid trailing space", "pending ", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := task.NewTaskStatus(tt.input)
			if !tt.isValid {
				if err == nil {
					t.Fatalf("expected error for input %q, got nil", tt.input)
				}
				if !errors.Is(err, task.ErrInvalidStatus) {
					t.Fatalf("expected ErrInvalidStatus, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected valid status %q, got error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}
