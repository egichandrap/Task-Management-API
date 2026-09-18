package project

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"task-api/internal/domain/project"
	"task-api/pkg/errors"
)

// fakeProjectRepo implements project.Repository in memory for unit tests.
type fakeProjectRepo struct {
	byID      map[string]*project.Project
	members   map[string]map[string]bool // projectID -> userID -> member
	memberLog map[string][]string        // projectID -> join order
	removed   [][2]string                // (projectID, userID) pairs
	added     [][2]string                // (projectID, userID) pairs
	errCreate error
	errFind   error
	errList   error
}

func newFakeProjectRepo() *fakeProjectRepo {
	return &fakeProjectRepo{
		byID:      map[string]*project.Project{},
		members:   map[string]map[string]bool{},
		memberLog: map[string][]string{},
	}
}

func (f *fakeProjectRepo) Create(_ context.Context, p *project.Project) error {
	if f.errCreate != nil {
		return f.errCreate
	}
	cp := *p
	f.byID[cp.ID] = &cp
	// Mirrors the real repository: project + owner membership commit together.
	f.addMembership(cp.ID, cp.OwnerID)
	return nil
}

func (f *fakeProjectRepo) FindByID(_ context.Context, id string) (*project.Project, error) {
	if f.errFind != nil {
		return nil, f.errFind
	}
	p, ok := f.byID[id]
	if !ok {
		return nil, project.ErrNotFound
	}
	return p, nil
}

func (f *fakeProjectRepo) FindAllByMember(_ context.Context, userID string) ([]project.Project, error) {
	if f.errList != nil {
		return nil, f.errList
	}
	out := make([]project.Project, 0)
	for _, p := range f.byID {
		if f.members[p.ID][userID] {
			out = append(out, *p)
		}
	}
	return out, nil
}

func (f *fakeProjectRepo) IsMember(_ context.Context, projectID, userID string) (bool, error) {
	return f.members[projectID][userID], nil
}

func (f *fakeProjectRepo) addMembership(projectID, userID string) {
	if f.members[projectID] == nil {
		f.members[projectID] = map[string]bool{}
	}
	if !f.members[projectID][userID] {
		f.members[projectID][userID] = true
		f.memberLog[projectID] = append(f.memberLog[projectID], userID)
	}
}

func (f *fakeProjectRepo) AddMember(_ context.Context, m *project.Member) error {
	if f.members[m.ProjectID][m.UserID] {
		return project.ErrAlreadyMember
	}
	f.addMembership(m.ProjectID, m.UserID)
	f.added = append(f.added, [2]string{m.ProjectID, m.UserID})
	return nil
}

func (f *fakeProjectRepo) RemoveMember(_ context.Context, projectID, userID string) error {
	delete(f.members[projectID], userID)
	f.removed = append(f.removed, [2]string{projectID, userID})
	return nil
}

func (f *fakeProjectRepo) ListMembers(_ context.Context, projectID string) ([]project.Member, error) {
	out := make([]project.Member, 0, len(f.memberLog[projectID]))
	for _, uid := range f.memberLog[projectID] {
		if f.members[projectID][uid] {
			out = append(out, project.Member{ProjectID: projectID, UserID: uid})
		}
	}
	return out, nil
}

// fakeUsers implements UserChecker in memory for unit tests.
type fakeUsers struct {
	exists bool
	err    error
}

func (f *fakeUsers) Exists(_ context.Context, _ string) (bool, error) {
	return f.exists, f.err
}

func newProjectUsecase(repo project.Repository) *projectUsecase {
	return newProjectUsecaseWithUsers(repo, &fakeUsers{exists: true})
}

func newProjectUsecaseWithUsers(repo project.Repository, users UserChecker) *projectUsecase {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(repo, users, log)
}

func seedProject(t *testing.T, repo *fakeProjectRepo, name, ownerID string) *project.Project {
	t.Helper()
	p, err := project.NewProject(name, ownerID)
	if err != nil {
		t.Fatalf("seed failed: %v", err)
	}
	if err := repo.Create(context.Background(), p); err != nil {
		t.Fatalf("seed failed: %v", err)
	}
	return p
}

func TestProjectUsecaseCreate(t *testing.T) {
	t.Run("creates a project and the owner is its first member", func(t *testing.T) {
		repo := newFakeProjectRepo()
		uc := newProjectUsecase(repo)

		created, err := uc.Create(context.Background(), "Alpha Team", "user-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if created.Name != "Alpha Team" || created.OwnerID != "user-1" {
			t.Fatalf("unexpected fields: %+v", created)
		}
		if len(repo.byID) != 1 {
			t.Fatalf("expected one project persisted, got %d", len(repo.byID))
		}
		if !repo.members[created.ID]["user-1"] {
			t.Fatal("expected owner to be a member")
		}
	})

	t.Run("invalid name is rejected without touching the repository", func(t *testing.T) {
		repo := newFakeProjectRepo()
		uc := newProjectUsecase(repo)

		_, err := uc.Create(context.Background(), "   ", "user-1")
		if err != customerrors.ErrBadRequest {
			t.Fatalf("expected ErrBadRequest, got %v", err)
		}
		if len(repo.byID) != 0 {
			t.Fatal("expected repository not to be called")
		}
	})

	t.Run("repository failure maps to internal server error", func(t *testing.T) {
		repo := newFakeProjectRepo()
		repo.errCreate = errors.New("db down")
		uc := newProjectUsecase(repo)

		_, err := uc.Create(context.Background(), "Alpha Team", "user-1")
		if err != customerrors.ErrInternalServer {
			t.Fatalf("expected ErrInternalServer, got %v", err)
		}
	})
}

func TestProjectUsecaseListMine(t *testing.T) {
	repo := newFakeProjectRepo()
	seedProject(t, repo, "Alpha", "user-1")
	seedProject(t, repo, "Beta", "user-2")
	uc := newProjectUsecase(repo)

	// user-2 joins Alpha.
	alphaID := keyOf(repo, "Alpha")
	if err := repo.AddMember(context.Background(), project.NewMember(alphaID, "user-2")); err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	got, err := uc.ListMine(context.Background(), "user-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 projects for user-2, got %d", len(got))
	}

	got, err = uc.ListMine(context.Background(), "user-3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 projects for user-3, got %d", len(got))
	}
}

// keyOf finds a seeded project id by name in the fake repo.
func keyOf(repo *fakeProjectRepo, name string) string {
	for id, p := range repo.byID {
		if p.Name == name {
			return id
		}
	}
	return ""
}

func TestProjectUsecaseDetail(t *testing.T) {
	t.Run("member can see the project", func(t *testing.T) {
		repo := newFakeProjectRepo()
		p := seedProject(t, repo, "Alpha", "user-1")
		uc := newProjectUsecase(repo)

		got, err := uc.Detail(context.Background(), p.ID, "user-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.ID != p.ID {
			t.Fatalf("expected project %q, got %q", p.ID, got.ID)
		}
	})

	t.Run("non-member is forbidden", func(t *testing.T) {
		repo := newFakeProjectRepo()
		p := seedProject(t, repo, "Alpha", "user-1")
		uc := newProjectUsecase(repo)

		_, err := uc.Detail(context.Background(), p.ID, "user-2")
		if err != customerrors.ErrForbidden {
			t.Fatalf("expected ErrForbidden, got %v", err)
		}
	})

	t.Run("missing project maps to not found", func(t *testing.T) {
		repo := newFakeProjectRepo()
		uc := newProjectUsecase(repo)

		_, err := uc.Detail(context.Background(), "missing", "user-1")
		if err != customerrors.ErrNotFound {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})
}

func TestProjectUsecaseAddMember(t *testing.T) {
	t.Run("owner adds an existing user as member", func(t *testing.T) {
		repo := newFakeProjectRepo()
		p := seedProject(t, repo, "Alpha", "user-1")
		uc := newProjectUsecase(repo)

		if err := uc.AddMember(context.Background(), p.ID, "user-1", "user-2"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !repo.members[p.ID]["user-2"] {
			t.Fatal("expected user-2 to be a member")
		}
	})

	t.Run("adding an existing member maps to conflict", func(t *testing.T) {
		repo := newFakeProjectRepo()
		p := seedProject(t, repo, "Alpha", "user-1")
		uc := newProjectUsecase(repo)

		err := uc.AddMember(context.Background(), p.ID, "user-1", "user-1")
		if err != customerrors.ErrConflict {
			t.Fatalf("expected ErrConflict, got %v", err)
		}
	})

	t.Run("non-owner is forbidden", func(t *testing.T) {
		repo := newFakeProjectRepo()
		p := seedProject(t, repo, "Alpha", "user-1")
		uc := newProjectUsecase(repo)

		err := uc.AddMember(context.Background(), p.ID, "user-2", "user-3")
		if err != customerrors.ErrForbidden {
			t.Fatalf("expected ErrForbidden, got %v", err)
		}
		if len(repo.added) != 0 {
			t.Fatal("expected no membership added")
		}
	})

	t.Run("target user that does not exist is rejected", func(t *testing.T) {
		repo := newFakeProjectRepo()
		p := seedProject(t, repo, "Alpha", "user-1")
		uc := newProjectUsecaseWithUsers(repo, &fakeUsers{exists: false})

		err := uc.AddMember(context.Background(), p.ID, "user-1", "user-ghost")
		if err != customerrors.ErrBadRequest {
			t.Fatalf("expected ErrBadRequest, got %v", err)
		}
		if len(repo.added) != 0 {
			t.Fatal("expected no membership added")
		}
	})

	t.Run("user existence check failure maps to internal server error", func(t *testing.T) {
		repo := newFakeProjectRepo()
		p := seedProject(t, repo, "Alpha", "user-1")
		uc := newProjectUsecaseWithUsers(repo, &fakeUsers{err: errors.New("db down")})

		err := uc.AddMember(context.Background(), p.ID, "user-1", "user-2")
		if err != customerrors.ErrInternalServer {
			t.Fatalf("expected ErrInternalServer, got %v", err)
		}
	})

	t.Run("missing project maps to not found", func(t *testing.T) {
		repo := newFakeProjectRepo()
		uc := newProjectUsecase(repo)

		err := uc.AddMember(context.Background(), "missing", "user-1", "user-2")
		if err != customerrors.ErrNotFound {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})
}

func TestProjectUsecaseRemoveMember(t *testing.T) {
	t.Run("owner removes a member", func(t *testing.T) {
		repo := newFakeProjectRepo()
		p := seedProject(t, repo, "Alpha", "user-1")
		if err := repo.AddMember(context.Background(), project.NewMember(p.ID, "user-2")); err != nil {
			t.Fatalf("seed failed: %v", err)
		}
		uc := newProjectUsecase(repo)

		if err := uc.RemoveMember(context.Background(), p.ID, "user-1", "user-2"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if repo.members[p.ID]["user-2"] {
			t.Fatal("expected user-2 to no longer be a member")
		}
	})

	t.Run("the owner cannot be removed", func(t *testing.T) {
		repo := newFakeProjectRepo()
		p := seedProject(t, repo, "Alpha", "user-1")
		uc := newProjectUsecase(repo)

		err := uc.RemoveMember(context.Background(), p.ID, "user-1", "user-1")
		if err != customerrors.ErrBadRequest {
			t.Fatalf("expected ErrBadRequest, got %v", err)
		}
		if !repo.members[p.ID]["user-1"] {
			t.Fatal("expected owner to remain a member")
		}
	})

	t.Run("non-owner is forbidden", func(t *testing.T) {
		repo := newFakeProjectRepo()
		p := seedProject(t, repo, "Alpha", "user-1")
		if err := repo.AddMember(context.Background(), project.NewMember(p.ID, "user-2")); err != nil {
			t.Fatalf("seed failed: %v", err)
		}
		uc := newProjectUsecase(repo)

		err := uc.RemoveMember(context.Background(), p.ID, "user-2", "user-1")
		if err != customerrors.ErrForbidden {
			t.Fatalf("expected ErrForbidden, got %v", err)
		}
		if !repo.members[p.ID]["user-1"] {
			t.Fatal("expected owner to remain a member")
		}
	})

	t.Run("removing a non-member is a no-op success", func(t *testing.T) {
		repo := newFakeProjectRepo()
		p := seedProject(t, repo, "Alpha", "user-1")
		uc := newProjectUsecase(repo)

		if err := uc.RemoveMember(context.Background(), p.ID, "user-1", "user-3"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestProjectUsecaseListMembers(t *testing.T) {
	t.Run("member lists members including the owner", func(t *testing.T) {
		repo := newFakeProjectRepo()
		p := seedProject(t, repo, "Alpha", "user-1")
		if err := repo.AddMember(context.Background(), project.NewMember(p.ID, "user-2")); err != nil {
			t.Fatalf("seed failed: %v", err)
		}
		uc := newProjectUsecase(repo)

		members, err := uc.ListMembers(context.Background(), p.ID, "user-2")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(members) != 2 {
			t.Fatalf("expected 2 members, got %d", len(members))
		}
		if members[0].UserID != "user-1" {
			t.Fatalf("expected owner first, got %q", members[0].UserID)
		}
	})

	t.Run("non-member is forbidden", func(t *testing.T) {
		repo := newFakeProjectRepo()
		p := seedProject(t, repo, "Alpha", "user-1")
		uc := newProjectUsecase(repo)

		_, err := uc.ListMembers(context.Background(), p.ID, "user-3")
		if err != customerrors.ErrForbidden {
			t.Fatalf("expected ErrForbidden, got %v", err)
		}
	})
}
