package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/hodynguyen/construct-flow/apps/gw-gateway/api/http/middleware"
	taskv1 "github.com/hodynguyen/construct-flow/gen/go/proto/task_service/v1"
)

// TaskHandler forwards task operations to task-service gRPC.
type TaskHandler struct {
	taskClient taskv1.TaskServiceClient
}

func NewTaskHandler(taskClient taskv1.TaskServiceClient) *TaskHandler {
	return &TaskHandler{taskClient: taskClient}
}

type createTaskRequest struct {
	Title       string `json:"title"      binding:"required"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
	AssignedTo  string `json:"assigned_to"`
	DueDate     string `json:"due_date"`
}

// CreateTask godoc
// @Summary Create a task in a project
// @Tags tasks
// @Security BearerAuth
// @Router /api/v1/projects/{project_id}/tasks [post]
func (h *TaskHandler) CreateTask(c *gin.Context) {
	userID, companyID, _, err := middleware.GetClaims(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	var req createTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := h.taskClient.CreateTask(c.Request.Context(), &taskv1.CreateTaskRequest{
		CompanyId:   companyID,
		ProjectId:   c.Param("project_id"),
		CreatedBy:   userID,
		Title:       req.Title,
		Description: req.Description,
		Priority:    req.Priority,
		AssignedTo:  req.AssignedTo,
		DueDate:     req.DueDate,
	})
	if err != nil {
		c.JSON(grpcToHTTPStatus(err), gin.H{"error": grpcMessage(err)})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"task": resp.Task})
}

// GetTask godoc
// @Summary Get a task by ID
// @Tags tasks
// @Security BearerAuth
// @Router /api/v1/tasks/{id} [get]
func (h *TaskHandler) GetTask(c *gin.Context) {
	_, companyID, _, err := middleware.GetClaims(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	resp, err := h.taskClient.GetTask(c.Request.Context(), &taskv1.GetTaskRequest{
		CompanyId: companyID,
		Id:        c.Param("id"),
	})
	if err != nil {
		c.JSON(grpcToHTTPStatus(err), gin.H{"error": grpcMessage(err)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"task": resp.Task})
}

// ListTasks godoc
// @Summary List tasks in a project
// @Tags tasks
// @Security BearerAuth
// @Router /api/v1/projects/{project_id}/tasks [get]
func (h *TaskHandler) ListTasks(c *gin.Context) {
	_, companyID, _, err := middleware.GetClaims(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	resp, err := h.taskClient.ListTasks(c.Request.Context(), &taskv1.ListTasksRequest{
		CompanyId:  companyID,
		ProjectId:  c.Param("project_id"),
		Status:     c.Query("status"),
		AssignedTo: c.Query("assigned_to"),
		Priority:   c.Query("priority"),
		Page:       int32(page),
		PageSize:   int32(pageSize),
	})
	if err != nil {
		c.JSON(grpcToHTTPStatus(err), gin.H{"error": grpcMessage(err)})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"tasks":     resp.Tasks,
		"total":     resp.Total,
		"page":      resp.Page,
		"page_size": resp.PageSize,
	})
}

type assignTaskRequest struct {
	AssignedTo string `json:"assigned_to" binding:"required"`
}

// AssignTask godoc
// @Summary Assign a task to a worker (manager only)
// @Tags tasks
// @Security BearerAuth
// @Router /api/v1/tasks/{id}/assign [post]
func (h *TaskHandler) AssignTask(c *gin.Context) {
	userID, companyID, _, err := middleware.GetClaims(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	var req assignTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := h.taskClient.AssignTask(c.Request.Context(), &taskv1.AssignTaskRequest{
		CompanyId:  companyID,
		TaskId:     c.Param("id"),
		AssignedTo: req.AssignedTo,
		AssignedBy: userID,
	})
	if err != nil {
		c.JSON(grpcToHTTPStatus(err), gin.H{"error": grpcMessage(err)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"task": resp.Task})
}

type updateStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

// UpdateTaskStatus godoc
// @Summary Update task status
// @Tags tasks
// @Security BearerAuth
// @Router /api/v1/tasks/{id}/status [patch]
func (h *TaskHandler) UpdateTaskStatus(c *gin.Context) {
	userID, companyID, role, err := middleware.GetClaims(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	var req updateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := h.taskClient.UpdateTaskStatus(c.Request.Context(), &taskv1.UpdateTaskStatusRequest{
		CompanyId: companyID,
		TaskId:    c.Param("id"),
		NewStatus: req.Status,
		ChangedBy: userID,
		Role:      role,
	})
	if err != nil {
		c.JSON(grpcToHTTPStatus(err), gin.H{"error": grpcMessage(err)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"task": resp.Task})
}
