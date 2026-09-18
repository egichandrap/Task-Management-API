package usecase

import (
	"context"
	"errors"
	"log/slog"

	"task-api/internal/domain/user"
	"task-api/pkg/errors"
	"task-api/pkg/utils"
)

// TokenIssuer is the application port for credential issuance.
// The usecase depends on this abstraction, not on the JWT library.
type TokenIssuer interface {
	Issue(userID string) (string, error)
}

// AuthUsecase is the application port consumed by the transport layer.
type AuthUsecase interface {
	Register(ctx context.Context, username, password string) (*user.User, error)
	Login(ctx context.Context, username, password string) (string, error)
}

type authUsecase struct {
	repo   user.Repository
	tokens TokenIssuer
	log    *slog.Logger
}

func NewAuthUsecase(repo user.Repository, tokens TokenIssuer, log *slog.Logger) AuthUsecase {
	return &authUsecase{repo: repo, tokens: tokens, log: log}
}

func (u *authUsecase) Register(ctx context.Context, username, password string) (*user.User, error) {
	// Value objects enforce the identity rules before anything is persisted.
	uname, err := user.NewUsername(username)
	if err != nil {
		return nil, customerrors.ErrBadRequest
	}
	if err := user.ValidatePassword(password); err != nil {
		return nil, customerrors.ErrBadRequest
	}

	hash, err := utils.HashPassword(password)
	if err != nil {
		u.log.Error("hash password failed", slog.Any("error", err))
		return nil, customerrors.ErrInternalServer
	}

	newUser := user.NewUser(uname, hash)
	if err := u.repo.Create(ctx, newUser); err != nil {
		if errors.Is(err, user.ErrUsernameTaken) {
			return nil, customerrors.ErrConflict
		}
		// Any other failure (e.g. database down) must not be reported as a conflict.
		u.log.Error("create user failed", slog.Any("error", err), slog.String("username", uname.String()))
		return nil, customerrors.ErrInternalServer
	}

	return newUser, nil
}

func (u *authUsecase) Login(ctx context.Context, username, password string) (string, error) {
	uname, err := user.NewUsername(username)
	if err != nil {
		return "", customerrors.ErrUnauthorized
	}

	existing, err := u.repo.GetByUsername(ctx, uname)
	if err != nil {
		if !errors.Is(err, user.ErrNotFound) {
			// Infrastructure failure: log it, but answer with the same generic
			// unauthorized to avoid leaking account existence or system state.
			u.log.Error("get user failed", slog.Any("error", err))
		}
		return "", customerrors.ErrUnauthorized
	}

	if !utils.CheckPasswordHash(password, existing.PasswordHash) {
		return "", customerrors.ErrUnauthorized
	}

	token, err := u.tokens.Issue(existing.ID)
	if err != nil {
		u.log.Error("issue token failed", slog.Any("error", err), slog.String("user_id", existing.ID))
		return "", customerrors.ErrInternalServer
	}

	return token, nil
}
