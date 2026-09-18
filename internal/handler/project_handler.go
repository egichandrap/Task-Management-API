package handler

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"task-api/internal/domain/project"
	"task-api/pkg/errors"
	"task-api/pkg/response"

	"github.com/gin-gonic/gin"
)

// ProjectUsecase is the driving port of the Project context. Following the
// Go idiom, it is defined at the consumer side with only the operations the
// handler needs; the application layer provides the implementation.
type ProjectUsecase interface {
	Create(ctx context.Context, name, ownerID string) (*project.Project, error)
	ListMine(ctx context.Context, userID string) ([]project.Project, error)
	Detail(ctx context.Context, id, userID string) (*project.Project, error)
	AddMember(ctx context.Context, projectID, ownerID, userID string) error
	RemoveMember(ctx context.Context, projectID, ownerID, userID string) error
	ListMembers(ctx context.Context, projectID, userID string) ([]project.Member, error)
}

type ProjectHandler struct {
	usecase ProjectUsecase
}

func NewProjectHandler(usecase ProjectUsecase) *ProjectHandler {
	return &ProjectHandler{usecase: usecase}
}

// projectResponse and memberResponse are transport DTOs. The domain
// entities carry no JSON tags, so the transport layer owns its own
// representation.
type projectResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	OwnerID   string    `json:"owner_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func toProjectResponse(p *project.Project) projectResponse {
	return projectResponse{
		ID:        p.ID,
		Name:      p.Name,
		OwnerID:   p.OwnerID,
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
}

func toProjectListResponse(projects []project.Project) []projectResponse {
	out := make([]projectResponse, 0, len(projects))
	for i := range projects {
		out = append(out, toProjectResponse(&projects[i]))
	}
	return out
}

type memberResponse struct {
	UserID   string    `json:"user_id"`
	JoinedAt time.Time `json:"joined_at"`
}

func toMemberListResponse(members []project.Member) []memberResponse {
	out := make([]memberResponse, 0, len(members))
	for _, m := range members {
		out = append(out, memberResponse{
			UserID:   m.UserID,
			JoinedAt: m.JoinedAt,
		})
	}
	return out
}

type CreateProjectReq struct {
	Name string `json:"name" binding:"required"`
}

func (h *ProjectHandler) Create(c *gin.Context) {
	var req CreateProjectReq
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("request body binding failed",
			slog.String("request_id", c.GetString("request_id")),
			slog.Any("error", err))
		c.Error(customerrors.ErrBadRequest)
		return
	}

	userID := c.GetString("user_id")

	created, err := h.usecase.Create(c.Request.Context(), req.Name, userID)
	if err != nil {
		c.Error(err)
		return
	}

	response.JSONSuccess(c, http.StatusCreated, toProjectResponse(created), nil)
}

func (h *ProjectHandler) ListMine(c *gin.Context) {
	userID := c.GetString("user_id")

	projects, err := h.usecase.ListMine(c.Request.Context(), userID)
	if err != nil {
		c.Error(err)
		return
	}

	response.JSONSuccess(c, http.StatusOK, toProjectListResponse(projects), nil)
}

func (h *ProjectHandler) Detail(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")

	found, err := h.usecase.Detail(c.Request.Context(), id, userID)
	if err != nil {
		c.Error(err)
		return
	}

	response.JSONSuccess(c, http.StatusOK, toProjectResponse(found), nil)
}

type AddMemberReq struct {
	UserID string `json:"user_id" binding:"required"`
}

func (h *ProjectHandler) AddMember(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")

	var req AddMemberReq
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("request body binding failed",
			slog.String("request_id", c.GetString("request_id")),
			slog.Any("error", err))
		c.Error(customerrors.ErrBadRequest)
		return
	}

	if err := h.usecase.AddMember(c.Request.Context(), id, userID, req.UserID); err != nil {
		c.Error(err)
		return
	}

	response.JSONSuccess(c, http.StatusCreated, gin.H{"message": "Member added successfully"}, nil)
}

func (h *ProjectHandler) RemoveMember(c *gin.Context) {
	id := c.Param("id")
	targetUserID := c.Param("userId")
	userID := c.GetString("user_id")

	if err := h.usecase.RemoveMember(c.Request.Context(), id, userID, targetUserID); err != nil {
		c.Error(err)
		return
	}

	response.JSONSuccess(c, http.StatusOK, gin.H{"message": "Member removed successfully"}, nil)
}

func (h *ProjectHandler) ListMembers(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")

	members, err := h.usecase.ListMembers(c.Request.Context(), id, userID)
	if err != nil {
		c.Error(err)
		return
	}

	response.JSONSuccess(c, http.StatusOK, toMemberListResponse(members), nil)
}
