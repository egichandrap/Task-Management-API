package user

import "errors"

// Domain errors of the identity (User) context.
var (
	ErrNotFound           = errors.New("user not found")
	ErrUsernameTaken      = errors.New("username already taken")
	ErrInvalidUsername    = errors.New("username must be 3-255 characters")
	ErrInvalidPassword    = errors.New("password must be 8-72 characters")
	ErrInvalidCredentials = errors.New("invalid username or password")
)
