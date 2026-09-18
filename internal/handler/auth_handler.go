package handler

import (
	"net/http"

	"task-api/internal/usecase"
	"task-api/pkg/errors"
	"task-api/pkg/response"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	usecase usecase.AuthUsecase
}

func NewAuthHandler(usecase usecase.AuthUsecase) *AuthHandler {
	return &AuthHandler{usecase: usecase}
}

type RegisterReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterReq
	if err := c.ShouldBindJSON(&req); err != nil {
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
