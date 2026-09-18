// Package project contains the application services of the Project bounded
// context. It orchestrates the Project aggregate and its repository port.
package project

import (
	"context"
	"errors"
	"log/slog"

	"task-api/internal/domain/project"
	"task-api/pkg/errors"
)

// UserChecker is the narrow port this service needs from the User context:
// verifying that a candidate member exists.
type UserChecker interface {
	Exists(ctx context.Context, id string) (bool, error)
}

type projectUsecase struct {
	repo  project.Repository
	users UserChecker
	log   *slog.Logger
}

// New builds the project application service. The driving port it satisfies
// is defined by the transport layer (consumer side), following the Go idiom
// of declaring interfaces where they are used.
func New(repo project.Repository, users UserChecker, log *slog.Logger) *projectUsecase {
	return &projectUsecase{repo: repo, users: users, log: log}
}

func (u *projectUsecase) Create(ctx context.Context, name, ownerID string) (*project.Project, error) {
	// The aggregate factory owns the "new project" rules (valid name).
	p, err := project.NewProject(name, ownerID)
	if err != nil {
		return nil, customerrors.ErrBadRequest
	}

	// The repository persists the project and the owner's membership in one
	// transaction ("owner is a member").
	if err := u.repo.Create(ctx, p); err != nil {
		u.log.Error("create project failed", slog.Any("error", err), slog.String("owner_id", ownerID))
		return nil, customerrors.ErrInternalServer
	}

	return p, nil
}

func (u *projectUsecase) ListMine(ctx context.Context, userID string) ([]project.Project, error) {
	projects, err := u.repo.FindAllByMember(ctx, userID)
	if err != nil {
		u.log.Error("list projects failed", slog.Any("error", err), slog.String("user_id", userID))
		return nil, customerrors.ErrInternalServer
	}
	return projects, nil
}

// findForMember loads the aggregate and enforces membership. Unlike the
// Task context (which hides ownership violations behind 404 to avoid
// leaking other users' resources), a project is a shared team surface, so
// non-members get an explicit 403.
func (u *projectUsecase) findForMember(ctx context.Context, id, userID string) (*project.Project, error) {
	p, err := u.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, project.ErrNotFound) {
			return nil, customerrors.ErrNotFound
		}
		u.log.Error("find project failed", slog.Any("error", err), slog.String("project_id", id))
		return nil, customerrors.ErrInternalServer
	}

	isMember, err := u.repo.IsMember(ctx, id, userID)
	if err != nil {
		u.log.Error("check membership failed", slog.Any("error", err), slog.String("project_id", id))
		return nil, customerrors.ErrInternalServer
	}
	if !isMember {
		return nil, customerrors.ErrForbidden
	}
	return p, nil
}

func (u *projectUsecase) Detail(ctx context.Context, id, userID string) (*project.Project, error) {
	return u.findForMember(ctx, id, userID)
}

func (u *projectUsecase) AddMember(ctx context.Context, projectID, ownerID, userID string) error {
	p, err := u.repo.FindByID(ctx, projectID)
	if err != nil {
		if errors.Is(err, project.ErrNotFound) {
			return customerrors.ErrNotFound
		}
		u.log.Error("find project failed", slog.Any("error", err), slog.String("project_id", projectID))
		return customerrors.ErrInternalServer
	}

	// Only the owner manages membership.
	if err := p.EnsureOwnedBy(ownerID); err != nil {
		return customerrors.ErrForbidden
	}

	// The target must be an existing user; otherwise the write would only
	// fail later on the FK constraint and surface as a 500.
	exists, err := u.users.Exists(ctx, userID)
	if err != nil {
		u.log.Error("check user failed", slog.Any("error", err), slog.String("user_id", userID))
		return customerrors.ErrInternalServer
	}
	if !exists {
		return customerrors.ErrBadRequest
	}

	if err := u.repo.AddMember(ctx, project.NewMember(p.ID, userID)); err != nil {
		if errors.Is(err, project.ErrAlreadyMember) {
			return customerrors.ErrConflict
		}
		u.log.Error("add member failed", slog.Any("error", err), slog.String("project_id", projectID))
		return customerrors.ErrInternalServer
	}

	return nil
}

func (u *projectUsecase) RemoveMember(ctx context.Context, projectID, ownerID, userID string) error {
	p, err := u.repo.FindByID(ctx, projectID)
	if err != nil {
		if errors.Is(err, project.ErrNotFound) {
			return customerrors.ErrNotFound
		}
		u.log.Error("find project failed", slog.Any("error", err), slog.String("project_id", projectID))
		return customerrors.ErrInternalServer
	}

	// Only the owner manages membership.
	if err := p.EnsureOwnedBy(ownerID); err != nil {
		return customerrors.ErrForbidden
	}

	// The owner cannot be removed: ownership would dangle.
	if userID == p.OwnerID {
		return customerrors.ErrBadRequest
	}

	if err := u.repo.RemoveMember(ctx, projectID, userID); err != nil {
		u.log.Error("remove member failed", slog.Any("error", err), slog.String("project_id", projectID))
		return customerrors.ErrInternalServer
	}

	return nil
}

func (u *projectUsecase) ListMembers(ctx context.Context, projectID, userID string) ([]project.Member, error) {
	if _, err := u.findForMember(ctx, projectID, userID); err != nil {
		return nil, err
	}

	members, err := u.repo.ListMembers(ctx, projectID)
	if err != nil {
		u.log.Error("list members failed", slog.Any("error", err), slog.String("project_id", projectID))
		return nil, customerrors.ErrInternalServer
	}
	return members, nil
}
