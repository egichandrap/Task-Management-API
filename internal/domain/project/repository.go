package project

import "context"

// Repository is the persistence port of the Project context.
// The domain defines the contract; infrastructure implements it.
type Repository interface {
	// Create persists the project AND the owner's membership in one
	// transaction (the "owner is a member" invariant).
	Create(ctx context.Context, p *Project) error
	FindByID(ctx context.Context, id string) (*Project, error)
	// FindAllByMember returns projects where userID is a member.
	FindAllByMember(ctx context.Context, userID string) ([]Project, error)
	IsMember(ctx context.Context, projectID, userID string) (bool, error)
	AddMember(ctx context.Context, m *Member) error
	// RemoveMember is idempotent: removing a non-member is a no-op.
	RemoveMember(ctx context.Context, projectID, userID string) error
	ListMembers(ctx context.Context, projectID string) ([]Member, error)
}
