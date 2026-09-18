package task

import "errors"

// Domain errors of the Task context. Application and transport layers
// translate these into their own error representations.
var (
	ErrNotFound        = errors.New("task not found")
	ErrForbidden       = errors.New("task is not owned by the user")
	ErrInvalidStatus   = errors.New("invalid task status")
	ErrInvalidTitle    = errors.New("task title must be 1-255 characters")
	ErrInvalidAssignee = errors.New("assignee id is required")
)
