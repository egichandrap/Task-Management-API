package project

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

const maxNameLength = 255

// Project is the aggregate root of the Project context. It owns its
// identity and the owner-only management invariant of its membership.
type Project struct {
	ID        string
	Name      string
	OwnerID   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Member is a child entity of the Project aggregate: a user's membership.
// It has no meaning outside its project.
type Member struct {
	ProjectID string
	UserID    string
	JoinedAt  time.Time
}

// NewProject is the factory of the aggregate. The repository persists the
// project and the owner's membership atomically ("owner is a member").
func NewProject(name, ownerID string) (*Project, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > maxNameLength {
		return nil, ErrInvalidName
	}

	now := time.Now()
	return &Project{
		ID:        uuid.NewString(),
		Name:      name,
		OwnerID:   ownerID,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func NewMember(projectID, userID string) *Member {
	return &Member{
		ProjectID: projectID,
		UserID:    userID,
		JoinedAt:  time.Now(),
	}
}

// EnsureOwnedBy enforces the owner-only management invariant (add/remove
// members). Membership itself is checked by the application service.
func (p *Project) EnsureOwnedBy(userID string) error {
	if p.OwnerID != userID {
		return ErrForbidden
	}
	return nil
}
