package handler

import (
	"task-api/internal/domain"
	"task-api/pkg/response"
	"task-api/pkg/errors"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

type TaskHandler struct {
	repo domain.TaskRepository
}

func NewTaskHandler(repo domain.TaskRepository) *TaskHandler {
	return &TaskHandler{repo: repo}
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
	
	task := &domain.Task{
		Title:       req.Title,
		Description: req.Description,
		Status:      "pending",
		AssigneeID:  userID,
	}
	
	if err := h.repo.Create(c.Request.Context(), task); err != nil {
		c.Error(customerrors.ErrInternalServer)
		return
	}
	
	response.JSONSuccess(c, http.StatusCreated, task, nil)
}

func (h *TaskHandler) List(c *gin.Context) {
	userID := c.GetString("user_id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	
	offset := (page - 1) * limit
	
	filter := domain.TaskFilter{
		Status:   c.Query("status"),
		Title:    c.Query("title"),
		Limit:    limit,
		Offset:   offset,
		Assignee: userID,
	}
	
	tasks, total, err := h.repo.FindAll(c.Request.Context(), filter)
	if err != nil {
		c.Error(customerrors.ErrInternalServer)
		return
	}
	
	response.JSONSuccess(c, http.StatusOK, tasks, gin.H{"total": total, "page": page, "limit": limit})
}

func (h *TaskHandler) Detail(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")
	
	task, err := h.repo.FindByID(c.Request.Context(), id)
	if err != nil {
		c.Error(customerrors.ErrNotFound)
		return
	}
	
	if task.AssigneeID != userID {
		c.Error(customerrors.ErrNotFound)
		return
	}
	
	response.JSONSuccess(c, http.StatusOK, task, nil)
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
	
	task, err := h.repo.FindByID(c.Request.Context(), id)
	if err != nil {
		c.Error(customerrors.ErrNotFound)
		return
	}
	
	if task.AssigneeID != userID {
		c.Error(customerrors.ErrNotFound)
		return
	}
	
	if req.Title != "" {
		task.Title = req.Title
	}
	if req.Description != "" {
		task.Description = req.Description
	}
	if req.Status != "" {
		task.Status = req.Status
	}
	
	if err := h.repo.Update(c.Request.Context(), task); err != nil {
		c.Error(customerrors.ErrInternalServer)
		return
	}
	
	response.JSONSuccess(c, http.StatusOK, task, nil)
}

func (h *TaskHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")
	
	task, err := h.repo.FindByID(c.Request.Context(), id)
	if err != nil || task.AssigneeID != userID {
		c.Error(customerrors.ErrNotFound)
		return
	}
	
	if err := h.repo.Delete(c.Request.Context(), id); err != nil {
		c.Error(customerrors.ErrInternalServer)
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
	
	task, err := h.repo.FindByID(c.Request.Context(), id)
	if err != nil || task.AssigneeID != userID {
		c.Error(customerrors.ErrNotFound)
		return
	}
	
	if err := h.repo.AssignTaskTx(c.Request.Context(), id, req.AssigneeID, userID); err != nil {
		c.Error(customerrors.ErrInternalServer)
		return
	}
	
	response.JSONSuccess(c, http.StatusOK, gin.H{"message": "Task assigned successfully"}, nil)
}
