package user

import "context"

// Repository is the persistence port of the identity context.
type Repository interface {
	Create(ctx context.Context, u *User) error
	GetByUsername(ctx context.Context, username Username) (*User, error)
}
