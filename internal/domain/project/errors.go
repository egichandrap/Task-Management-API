package project

import "errors"

// Domain errors of the Project context. Application and transport layers
// translate these into their own error representations.
var (
	ErrNotFound      = errors.New("project not found")
	ErrForbidden     = errors.New("project is not owned by the user")
	ErrInvalidName   = errors.New("project name must be 1-255 characters")
	ErrAlreadyMember = errors.New("user is already a project member")
	ErrOwnerRemoval  = errors.New("project owner cannot be removed")
)
