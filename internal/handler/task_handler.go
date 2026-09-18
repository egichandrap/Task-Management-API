package handler

import (
	"net/http"
	"strconv"
	"time"

	"task-api/internal/domain/task"
	"task-api/internal/usecase"
	"task-api/pkg/errors"
	"task-api/pkg/response"

	"github.com/gin-gonic/gin"
)

type TaskHandler struct {
	usecase usecase.TaskUsecase
}

func NewTaskHandler(usecase usecase.TaskUsecase) *TaskHandler {
	return &TaskHandler{usecase: usecase}
}

// taskResponse is the transport DTO. The domain entity no longer carries
// JSON tags, so the transport layer owns its own representation.
type taskResponse struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	AssigneeID  string    `json:"assignee_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func toTaskResponse(t *task.Task) taskResponse {
	return taskResponse{
		ID:          t.ID,
		Title:       t.Title,
		Description: t.Description,
		Status:      t.Status.String(),
		AssigneeID:  t.AssigneeID,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}

func toTaskListResponse(tasks []task.Task) []taskResponse {
	out := make([]taskResponse, 0, len(tasks))
	for i := range tasks {
		out = append(out, toTaskResponse(&tasks[i]))
	}
	return out
}

type CreateTaskReq struct {
	Title       string `json:"title" binding:"required"`
	Description string `json:"description"`
}

func (h *TaskHandler) Create(c *gin.Context) {
	var req CreateTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(customerrors.ErrBadRequest)
		return
	}

	userID := c.GetString("user_id")

	created, err := h.usecase.Create(c.Request.Context(), req.Title, req.Description, userID)
	if err != nil {
		c.Error(err)
		return
	}

	response.JSONSuccess(c, http.StatusCreated, toTaskResponse(created), nil)
}

func (h *TaskHandler) List(c *gin.Context) {
	userID := c.GetString("user_id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))

	offset := (page - 1) * limit

	filter := task.Filter{
		Status:   c.Query("status"),
		Title:    c.Query("title"),
		Limit:    limit,
		Offset:   offset,
		Assignee: userID,
	}

	tasks, total, err := h.usecase.List(c.Request.Context(), filter)
	if err != nil {
		c.Error(err)
		return
	}

	response.JSONSuccess(c, http.StatusOK, toTaskListResponse(tasks), gin.H{"total": total, "page": page, "limit": limit})
}

func (h *TaskHandler) Detail(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")

	found, err := h.usecase.Detail(c.Request.Context(), id, userID)
	if err != nil {
		c.Error(err)
		return
	}

	response.JSONSuccess(c, http.StatusOK, toTaskResponse(found), nil)
}

type UpdateTaskReq struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

func (h *TaskHandler) Update(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")

	var req UpdateTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(customerrors.ErrBadRequest)
		return
	}

	updated, err := h.usecase.Update(c.Request.Context(), id, userID, req.Title, req.Description, req.Status)
	if err != nil {
		c.Error(err)
		return
	}

	response.JSONSuccess(c, http.StatusOK, toTaskResponse(updated), nil)
}

func (h *TaskHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")

	if err := h.usecase.Delete(c.Request.Context(), id, userID); err != nil {
		c.Error(err)
		return
	}

	response.JSONSuccess(c, http.StatusOK, nil, nil)
}

type AssignTaskReq struct {
	AssigneeID string `json:"assignee_id" binding:"required"`
}

func (h *TaskHandler) Assign(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")

	var req AssignTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(customerrors.ErrBadRequest)
		return
	}

	if err := h.usecase.Assign(c.Request.Context(), id, req.AssigneeID, userID); err != nil {
		c.Error(err)
		return
	}

	response.JSONSuccess(c, http.StatusOK, gin.H{"message": "Task assigned successfully"}, nil)
}
