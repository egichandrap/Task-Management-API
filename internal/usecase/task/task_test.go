package task

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"task-api/internal/domain/task"
	"task-api/pkg/errors"
)

// fakeTaskRepo implements task.Repository in memory for unit tests.
type fakeTaskRepo struct {
	byID       map[string]*task.Task
	created    []*task.Task
	updated    []*task.Task
	deleted    []string
	assigned   []*task.Task
	assignLogs []*task.TaskLog
	errCreate  error
	errFind    error
	errUpdate  error
	errDelete  error
	errAssign  error
}

func newFakeTaskRepo() *fakeTaskRepo {
	return &fakeTaskRepo{byID: map[string]*task.Task{}}
}

func (f *fakeTaskRepo) Create(_ context.Context, t *task.Task) error {
	if f.errCreate != nil {
		return f.errCreate
	}
	cp := *t
	f.created = append(f.created, &cp)
	f.byID[cp.ID] = &cp
	return nil
}

func (f *fakeTaskRepo) FindAll(_ context.Context, _ task.Filter) ([]task.Task, int64, error) {
	if f.errFind != nil {
		return nil, 0, f.errFind
	}
	out := make([]task.Task, 0, len(f.byID))
	for _, t := range f.byID {
		out = append(out, *t)
	}
	return out, int64(len(out)), nil
}

func (f *fakeTaskRepo) FindByID(_ context.Context, id string) (*task.Task, error) {
	if f.errFind != nil {
		return nil, f.errFind
	}
	t, ok := f.byID[id]
	if !ok {
		return nil, task.ErrNotFound
	}
	return t, nil
}

func (f *fakeTaskRepo) Update(_ context.Context, t *task.Task) error {
	if f.errUpdate != nil {
		return f.errUpdate
	}
	cp := *t
	f.updated = append(f.updated, &cp)
	f.byID[cp.ID] = &cp
	return nil
}

func (f *fakeTaskRepo) Delete(_ context.Context, id string) error {
	if f.errDelete != nil {
		return f.errDelete
	}
	delete(f.byID, id)
	f.deleted = append(f.deleted, id)
	return nil
}

func (f *fakeTaskRepo) SaveAssignment(_ context.Context, t *task.Task, log *task.TaskLog) error {
	if f.errAssign != nil {
		return f.errAssign
	}
	cp := *t
	f.assigned = append(f.assigned, &cp)
	f.assignLogs = append(f.assignLogs, log)
	f.byID[cp.ID] = &cp
	return nil
}

// fakeUsers implements UserChecker in memory for unit tests.
type fakeUsers struct {
	exists bool
	err    error
}

func (f *fakeUsers) Exists(_ context.Context, _ string) (bool, error) {
	return f.exists, f.err
}

// fakeProjects implements MembershipChecker in memory for unit tests.
type fakeProjects struct {
	member map[string]map[string]bool // projectID -> userID -> member
	err    error
}

func newFakeProjects() *fakeProjects {
	return &fakeProjects{member: map[string]map[string]bool{}}
}

func (f *fakeProjects) addMember(projectID, userID string) {
	if f.member[projectID] == nil {
		f.member[projectID] = map[string]bool{}
	}
	f.member[projectID][userID] = true
}

func (f *fakeProjects) IsMember(_ context.Context, projectID, userID string) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return f.member[projectID][userID], nil
}

// defaultProjects seeds the project used by seedTask with its usual cast:
// user-1, user-2, and user-3 are all members of "project-1".
func defaultProjects() *fakeProjects {
	projects := newFakeProjects()
	for _, u := range []string{"user-1", "user-2", "user-3"} {
		projects.addMember("project-1", u)
	}
	return projects
}

func newTaskUsecase(repo task.Repository) *taskUsecase {
	return newTaskUsecaseWith(repo, &fakeUsers{exists: true}, defaultProjects())
}

func newTaskUsecaseWithUsers(repo task.Repository, users UserChecker) *taskUsecase {
	return newTaskUsecaseWith(repo, users, defaultProjects())
}

func newTaskUsecaseWith(repo task.Repository, users UserChecker, projects MembershipChecker) *taskUsecase {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(repo, users, projects, log)
}

func seedTask(t *testing.T, repo *fakeTaskRepo, assignee string) *task.Task {
	t.Helper()
	tsk, err := task.NewTask("Old title", "Old description", assignee, "project-1")
	if err != nil {
		t.Fatalf("seed failed: %v", err)
	}
	repo.byID[tsk.ID] = tsk
	return tsk
}

func TestTaskUsecaseCreate(t *testing.T) {
	t.Run("creates a pending task via the aggregate factory", func(t *testing.T) {
		repo := newFakeTaskRepo()
		uc := newTaskUsecase(repo)

		created, err := uc.Create(context.Background(), "Write report", "Quarterly report", "user-1", "project-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if created.Status != task.StatusPending {
			t.Fatalf("expected pending, got %q", created.Status)
		}
		if created.ProjectID != "project-1" {
			t.Fatalf("expected project project-1, got %q", created.ProjectID)
		}
		if len(repo.created) != 1 {
			t.Fatalf("expected repository create to be called once, got %d", len(repo.created))
		}
	})

	t.Run("invalid title is rejected without touching the repository", func(t *testing.T) {
		repo := newFakeTaskRepo()
		uc := newTaskUsecase(repo)

		_, err := uc.Create(context.Background(), "   ", "", "user-1", "project-1")
		if err != customerrors.ErrBadRequest {
			t.Fatalf("expected ErrBadRequest, got %v", err)
		}
		if len(repo.created) != 0 {
			t.Fatal("expected repository not to be called")
		}
	})

	t.Run("missing project is rejected", func(t *testing.T) {
		repo := newFakeTaskRepo()
		uc := newTaskUsecase(repo)

		_, err := uc.Create(context.Background(), "Write report", "", "user-1", "")
		if err != customerrors.ErrBadRequest {
			t.Fatalf("expected ErrBadRequest, got %v", err)
		}
	})

	t.Run("creator who is not a project member is forbidden", func(t *testing.T) {
		repo := newFakeTaskRepo()
		uc := newTaskUsecaseWith(repo, &fakeUsers{exists: true}, newFakeProjects())

		_, err := uc.Create(context.Background(), "Write report", "", "user-1", "project-1")
		if err != customerrors.ErrForbidden {
			t.Fatalf("expected ErrForbidden, got %v", err)
		}
		if len(repo.created) != 0 {
			t.Fatal("expected repository not to be called")
		}
	})

	t.Run("membership check failure maps to internal server error", func(t *testing.T) {
		repo := newFakeTaskRepo()
		broken := &fakeProjects{err: errors.New("db down")}
		uc := newTaskUsecaseWith(repo, &fakeUsers{exists: true}, broken)

		_, err := uc.Create(context.Background(), "Write report", "", "user-1", "project-1")
		if err != customerrors.ErrInternalServer {
			t.Fatalf("expected ErrInternalServer, got %v", err)
		}
		if len(repo.created) != 0 {
			t.Fatal("expected repository not to be called")
		}
	})

	t.Run("repository failure maps to internal server error", func(t *testing.T) {
		repo := newFakeTaskRepo()
		repo.errCreate = errors.New("db down")
		uc := newTaskUsecase(repo)

		_, err := uc.Create(context.Background(), "Write report", "", "user-1", "project-1")
		if err != customerrors.ErrInternalServer {
			t.Fatalf("expected ErrInternalServer, got %v", err)
		}
	})
}

func TestTaskUsecaseDetail(t *testing.T) {
	t.Run("owner can see the task", func(t *testing.T) {
		repo := newFakeTaskRepo()
		tsk := seedTask(t, repo, "user-1")
		uc := newTaskUsecase(repo)

		got, err := uc.Detail(context.Background(), tsk.ID, "user-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.ID != tsk.ID {
			t.Fatalf("expected task %q, got %q", tsk.ID, got.ID)
		}
	})

	t.Run("non-owner gets not found so existence is not leaked", func(t *testing.T) {
		repo := newFakeTaskRepo()
		tsk := seedTask(t, repo, "user-1")
		uc := newTaskUsecase(repo)

		_, err := uc.Detail(context.Background(), tsk.ID, "user-2")
		if err != customerrors.ErrNotFound {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("missing task maps domain not found", func(t *testing.T) {
		repo := newFakeTaskRepo()
		uc := newTaskUsecase(repo)

		_, err := uc.Detail(context.Background(), "missing", "user-1")
		if err != customerrors.ErrNotFound {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("infrastructure failure maps to internal server error", func(t *testing.T) {
		repo := newFakeTaskRepo()
		repo.errFind = errors.New("db down")
		uc := newTaskUsecase(repo)

		_, err := uc.Detail(context.Background(), "any", "user-1")
		if err != customerrors.ErrInternalServer {
			t.Fatalf("expected ErrInternalServer, got %v", err)
		}
	})
}

func TestTaskUsecaseUpdate(t *testing.T) {
	t.Run("applies partial update through the aggregate", func(t *testing.T) {
		repo := newFakeTaskRepo()
		tsk := seedTask(t, repo, "user-1")
		uc := newTaskUsecase(repo)

		updated, err := uc.Update(context.Background(), tsk.ID, "user-1", "New title", "", "completed")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated.Title != "New title" {
			t.Fatalf("expected title updated, got %q", updated.Title)
		}
		if updated.Description != "Old description" {
			t.Fatalf("expected description unchanged, got %q", updated.Description)
		}
		if updated.Status != task.StatusCompleted {
			t.Fatalf("expected status completed, got %q", updated.Status)
		}
		if len(repo.updated) != 1 {
			t.Fatalf("expected repository update once, got %d", len(repo.updated))
		}
	})

	t.Run("invalid status is rejected without persisting", func(t *testing.T) {
		repo := newFakeTaskRepo()
		tsk := seedTask(t, repo, "user-1")
		uc := newTaskUsecase(repo)

		_, err := uc.Update(context.Background(), tsk.ID, "user-1", "", "", "hacked")
		if err != customerrors.ErrBadRequest {
			t.Fatalf("expected ErrBadRequest, got %v", err)
		}
		if len(repo.updated) != 0 {
			t.Fatal("expected repository not to be called")
		}
	})

	t.Run("non-owner gets not found", func(t *testing.T) {
		repo := newFakeTaskRepo()
		tsk := seedTask(t, repo, "user-1")
		uc := newTaskUsecase(repo)

		_, err := uc.Update(context.Background(), tsk.ID, "user-2", "New title", "", "")
		if err != customerrors.ErrNotFound {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})
}

func TestTaskUsecaseAssign(t *testing.T) {
	t.Run("assignee can reassign and an audit log is persisted", func(t *testing.T) {
		repo := newFakeTaskRepo()
		tsk := seedTask(t, repo, "user-1")
		uc := newTaskUsecase(repo)

		if err := uc.Assign(context.Background(), tsk.ID, "user-2", "user-1"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(repo.assigned) != 1 {
			t.Fatalf("expected one assignment persist, got %d", len(repo.assigned))
		}
		if repo.assigned[0].AssigneeID != "user-2" {
			t.Fatalf("expected persisted assignee user-2, got %q", repo.assigned[0].AssigneeID)
		}
		if repo.assignLogs[0].OldValue != "user-1" || repo.assignLogs[0].NewValue != "user-2" {
			t.Fatalf("unexpected audit log: %+v", repo.assignLogs[0])
		}
	})

	t.Run("a project member who is not the assignee can reassign", func(t *testing.T) {
		repo := newFakeTaskRepo()
		tsk := seedTask(t, repo, "user-1")
		uc := newTaskUsecase(repo)

		if err := uc.Assign(context.Background(), tsk.ID, "user-3", "user-2"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(repo.assigned) != 1 {
			t.Fatalf("expected one assignment persist, got %d", len(repo.assigned))
		}
		if repo.assigned[0].AssigneeID != "user-3" {
			t.Fatalf("expected persisted assignee user-3, got %q", repo.assigned[0].AssigneeID)
		}
		if repo.assignLogs[0].ChangedBy != "user-2" {
			t.Fatalf("expected changed_by user-2, got %q", repo.assignLogs[0].ChangedBy)
		}
	})

	t.Run("a non-member cannot assign", func(t *testing.T) {
		repo := newFakeTaskRepo()
		tsk := seedTask(t, repo, "user-1")
		projects := defaultProjects()
		uc := newTaskUsecaseWith(repo, &fakeUsers{exists: true}, projects)

		// user-outsider is an existing user but not a member of project-1.
		if err := uc.Assign(context.Background(), tsk.ID, "user-3", "user-outsider"); err != customerrors.ErrForbidden {
			t.Fatalf("expected ErrForbidden, got %v", err)
		}
		if len(repo.assigned) != 0 {
			t.Fatal("expected no assignment persist")
		}
	})

	t.Run("assigning to a non-existent user is rejected without persisting", func(t *testing.T) {
		repo := newFakeTaskRepo()
		tsk := seedTask(t, repo, "user-1")
		uc := newTaskUsecaseWithUsers(repo, &fakeUsers{exists: false})

		err := uc.Assign(context.Background(), tsk.ID, "user-ghost", "user-1")
		if err != customerrors.ErrBadRequest {
			t.Fatalf("expected ErrBadRequest, got %v", err)
		}
		if len(repo.assigned) != 0 {
			t.Fatal("expected no assignment persist")
		}
	})

	t.Run("assigning to a non-member of the project is rejected", func(t *testing.T) {
		repo := newFakeTaskRepo()
		tsk := seedTask(t, repo, "user-1")
		uc := newTaskUsecase(repo)

		// user-outsider exists but is not a member of project-1.
		err := uc.Assign(context.Background(), tsk.ID, "user-outsider", "user-1")
		if err != customerrors.ErrBadRequest {
			t.Fatalf("expected ErrBadRequest, got %v", err)
		}
		if len(repo.assigned) != 0 {
			t.Fatal("expected no assignment persist")
		}
	})

	t.Run("assignee existence check failure maps to internal server error", func(t *testing.T) {
		repo := newFakeTaskRepo()
		tsk := seedTask(t, repo, "user-1")
		uc := newTaskUsecaseWithUsers(repo, &fakeUsers{err: errors.New("db down")})

		err := uc.Assign(context.Background(), tsk.ID, "user-2", "user-1")
		if err != customerrors.ErrInternalServer {
			t.Fatalf("expected ErrInternalServer, got %v", err)
		}
		if len(repo.assigned) != 0 {
			t.Fatal("expected no assignment persist")
		}
	})

	t.Run("membership check failure maps to internal server error", func(t *testing.T) {
		repo := newFakeTaskRepo()
		tsk := seedTask(t, repo, "user-1")
		broken := &fakeProjects{err: errors.New("db down")}
		uc := newTaskUsecaseWith(repo, &fakeUsers{exists: true}, broken)

		err := uc.Assign(context.Background(), tsk.ID, "user-2", "user-1")
		if err != customerrors.ErrInternalServer {
			t.Fatalf("expected ErrInternalServer, got %v", err)
		}
		if len(repo.assigned) != 0 {
			t.Fatal("expected no assignment persist")
		}
	})
}

func TestTaskUsecaseDelete(t *testing.T) {
	t.Run("owner can delete", func(t *testing.T) {
		repo := newFakeTaskRepo()
		tsk := seedTask(t, repo, "user-1")
		uc := newTaskUsecase(repo)

		if err := uc.Delete(context.Background(), tsk.ID, "user-1"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(repo.deleted) != 1 {
			t.Fatalf("expected one delete, got %d", len(repo.deleted))
		}
	})

	t.Run("non-owner gets not found", func(t *testing.T) {
		repo := newFakeTaskRepo()
		tsk := seedTask(t, repo, "user-1")
		uc := newTaskUsecase(repo)

		if err := uc.Delete(context.Background(), tsk.ID, "user-2"); err != customerrors.ErrNotFound {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})
}

func TestTaskUsecaseList(t *testing.T) {
	repo := newFakeTaskRepo()
	seedTask(t, repo, "user-1")
	uc := newTaskUsecase(repo)

	tasks, total, err := uc.List(context.Background(), task.Filter{Assignee: "user-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 || len(tasks) != 1 {
		t.Fatalf("expected 1 task, got total=%d len=%d", total, len(tasks))
	}
}
