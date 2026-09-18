package usecase_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"task-api/internal/domain/user"
	"task-api/internal/usecase"
	"task-api/pkg/errors"
	"task-api/pkg/utils"
)

// fakeUserRepo implements user.Repository in memory for unit tests.
type fakeUserRepo struct {
	byUsername map[string]*user.User
	created    []*user.User
	errCreate  error
	errGet     error
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{byUsername: map[string]*user.User{}}
}

func (f *fakeUserRepo) Create(_ context.Context, u *user.User) error {
	if f.errCreate != nil {
		return f.errCreate
	}
	cp := *u
	f.created = append(f.created, &cp)
	f.byUsername[cp.Username.String()] = &cp
	return nil
}

func (f *fakeUserRepo) GetByUsername(_ context.Context, username user.Username) (*user.User, error) {
	if f.errGet != nil {
		return nil, f.errGet
	}
	u, ok := f.byUsername[username.String()]
	if !ok {
		return nil, user.ErrNotFound
	}
	return u, nil
}

// fakeTokens implements usecase.TokenIssuer.
type fakeTokens struct {
	err    error
	issued []string
}

func (f *fakeTokens) Issue(userID string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	f.issued = append(f.issued, userID)
	return "token-" + userID, nil
}

func newAuthUsecase(repo user.Repository, tokens usecase.TokenIssuer) usecase.AuthUsecase {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return usecase.NewAuthUsecase(repo, tokens, log)
}

func TestAuthUsecaseRegister(t *testing.T) {
	t.Run("registers a user with a hashed password", func(t *testing.T) {
		repo := newFakeUserRepo()
		uc := newAuthUsecase(repo, &fakeTokens{})

		created, err := uc.Register(context.Background(), "alice", "password123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if created.ID == "" {
			t.Fatal("expected domain to generate the id")
		}
		if created.PasswordHash == "password123" {
			t.Fatal("expected password to be stored as a hash")
		}
		if !utils.CheckPasswordHash("password123", created.PasswordHash) {
			t.Fatal("expected hash to verify against the plaintext password")
		}
		if len(repo.created) != 1 {
			t.Fatalf("expected one create, got %d", len(repo.created))
		}
	})

	t.Run("invalid username is rejected without touching the repository", func(t *testing.T) {
		repo := newFakeUserRepo()
		uc := newAuthUsecase(repo, &fakeTokens{})

		_, err := uc.Register(context.Background(), "ab", "password123")
		if err != customerrors.ErrBadRequest {
			t.Fatalf("expected ErrBadRequest, got %v", err)
		}
		if len(repo.created) != 0 {
			t.Fatal("expected repository not to be called")
		}
	})

	t.Run("short password is rejected", func(t *testing.T) {
		repo := newFakeUserRepo()
		uc := newAuthUsecase(repo, &fakeTokens{})

		_, err := uc.Register(context.Background(), "alice", "short")
		if err != customerrors.ErrBadRequest {
			t.Fatalf("expected ErrBadRequest, got %v", err)
		}
	})

	t.Run("taken username maps to conflict", func(t *testing.T) {
		repo := newFakeUserRepo()
		repo.errCreate = user.ErrUsernameTaken
		uc := newAuthUsecase(repo, &fakeTokens{})

		_, err := uc.Register(context.Background(), "alice", "password123")
		if err != customerrors.ErrConflict {
			t.Fatalf("expected ErrConflict, got %v", err)
		}
	})

	t.Run("infrastructure failure maps to internal server error, not conflict", func(t *testing.T) {
		repo := newFakeUserRepo()
		repo.errCreate = errors.New("db down")
		uc := newAuthUsecase(repo, &fakeTokens{})

		_, err := uc.Register(context.Background(), "alice", "password123")
		if err != customerrors.ErrInternalServer {
			t.Fatalf("expected ErrInternalServer, got %v", err)
		}
	})
}

func TestAuthUsecaseLogin(t *testing.T) {
	t.Run("issues a token for the user on valid credentials", func(t *testing.T) {
		repo := newFakeUserRepo()
		tokens := &fakeTokens{}
		uc := newAuthUsecase(repo, tokens)

		registered, err := uc.Register(context.Background(), "alice", "password123")
		if err != nil {
			t.Fatalf("seed failed: %v", err)
		}

		token, err := uc.Login(context.Background(), "alice", "password123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if token != "token-"+registered.ID {
			t.Fatalf("expected token for user %q, got %q", registered.ID, token)
		}
	})

	t.Run("unknown user gets unauthorized without enumeration", func(t *testing.T) {
		repo := newFakeUserRepo()
		uc := newAuthUsecase(repo, &fakeTokens{})

		_, err := uc.Login(context.Background(), "ghost", "password123")
		if err != customerrors.ErrUnauthorized {
			t.Fatalf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("wrong password gets unauthorized", func(t *testing.T) {
		repo := newFakeUserRepo()
		uc := newAuthUsecase(repo, &fakeTokens{})

		if _, err := uc.Register(context.Background(), "alice", "password123"); err != nil {
			t.Fatalf("seed failed: %v", err)
		}

		_, err := uc.Login(context.Background(), "alice", "wrongpassword")
		if err != customerrors.ErrUnauthorized {
			t.Fatalf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("token issuer failure maps to internal server error", func(t *testing.T) {
		repo := newFakeUserRepo()
		tokens := &fakeTokens{err: errors.New("signing key missing")}
		uc := newAuthUsecase(repo, tokens)

		if _, err := uc.Register(context.Background(), "alice", "password123"); err != nil {
			t.Fatalf("seed failed: %v", err)
		}

		_, err := uc.Login(context.Background(), "alice", "password123")
		if err != customerrors.ErrInternalServer {
			t.Fatalf("expected ErrInternalServer, got %v", err)
		}
	})
}
