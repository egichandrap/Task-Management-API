package customerrors

import "fmt"

type AppError struct {
	Code    string
	Message string
	Status  int
}

func (e *AppError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func New(status int, code, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Status:  status,
	}
}

var (
	ErrUnauthorized   = New(401, "UNAUTHORIZED", "Unauthorized access")
	ErrForbidden      = New(403, "FORBIDDEN", "Access to this resource is forbidden")
	ErrNotFound       = New(404, "NOT_FOUND", "Resource not found")
	ErrBadRequest     = New(400, "BAD_REQUEST", "Invalid request parameters")
	ErrInternalServer = New(500, "INTERNAL_SERVER_ERROR", "An unexpected error occurred")
	ErrConflict       = New(409, "CONFLICT", "Resource already exists")
)
