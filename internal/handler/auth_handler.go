package handler

import (
	"task-api/internal/domain"
	"task-api/internal/config"
	"task-api/pkg/utils"
	"task-api/pkg/response"
	"task-api/pkg/errors"
	"github.com/gin-gonic/gin"
	"net/http"
)

type AuthHandler struct {
	repo domain.UserRepository
	cfg  *config.Config
}

func NewAuthHandler(repo domain.UserRepository, cfg *config.Config) *AuthHandler {
	return &AuthHandler{repo: repo, cfg: cfg}
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
	
	hash, err := utils.HashPassword(req.Password)
	if err != nil {
		c.Error(customerrors.ErrInternalServer)
		return
	}
	
	user := &domain.User{
		Username: req.Username,
		Password: hash,
	}
	
	if err := h.repo.Create(c.Request.Context(), user); err != nil {
		c.Error(customerrors.ErrConflict)
		return
	}
	
	response.JSONSuccess(c, http.StatusCreated, gin.H{"id": user.ID, "username": user.Username}, nil)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req RegisterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(customerrors.ErrBadRequest)
		return
	}
	
	user, err := h.repo.GetByUsername(c.Request.Context(), req.Username)
	if err != nil {
		c.Error(customerrors.ErrUnauthorized)
		return
	}
	
	if !utils.CheckPasswordHash(req.Password, user.Password) {
		c.Error(customerrors.ErrUnauthorized)
		return
	}
	
	token, err := utils.GenerateJWT(user.ID, h.cfg.JWTSecret)
	if err != nil {
		c.Error(customerrors.ErrInternalServer)
		return
	}
	
	response.JSONSuccess(c, http.StatusOK, gin.H{"token": token}, nil)
}
