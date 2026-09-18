package handler

import (
	"context"
	"log/slog"
	"net/http"

	"task-api/internal/domain/user"
	"task-api/pkg/errors"
	"task-api/pkg/response"

	"github.com/gin-gonic/gin"
)

// AuthUsecase is the driving port of the User context. Following the Go
// idiom, it is defined at the consumer side with only the operations the
// handler needs; the application layer provides the implementation.
type AuthUsecase interface {
	Register(ctx context.Context, username, password string) (*user.User, error)
	Login(ctx context.Context, username, password string) (string, error)
}

type AuthHandler struct {
	usecase AuthUsecase
}

func NewAuthHandler(usecase AuthUsecase) *AuthHandler {
	return &AuthHandler{usecase: usecase}
}

type RegisterReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("request body binding failed",
			slog.String("request_id", c.GetString("request_id")),
			slog.Any("error", err))
		c.Error(customerrors.ErrBadRequest)
		return
	}

	registered, err := h.usecase.Register(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		c.Error(err)
		return
	}

	response.JSONSuccess(c, http.StatusCreated, gin.H{"id": registered.ID, "username": registered.Username.String()}, nil)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req RegisterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("request body binding failed",
			slog.String("request_id", c.GetString("request_id")),
			slog.Any("error", err))
		c.Error(customerrors.ErrBadRequest)
		return
	}

	token, err := h.usecase.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		c.Error(err)
		return
	}

	response.JSONSuccess(c, http.StatusOK, gin.H{"token": token}, nil)
}
