package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/hodynguyen/construct-flow/apps/gw-gateway/api/http/middleware"
	taskv1 "github.com/hodynguyen/construct-flow/gen/go/proto/task_service/v1"
)

// ProjectHandler forwards project CRUD to task-service gRPC.
type ProjectHandler struct {
	taskClient taskv1.TaskServiceClient
}

func NewProjectHandler(taskClient taskv1.TaskServiceClient) *ProjectHandler {
	return &ProjectHandler{taskClient: taskClient}
}

type createProjectRequest struct {
	Name        string `json:"name"        binding:"required"`
	Description string `json:"description"`
	StartDate   string `json:"start_date"`
	EndDate     string `json:"end_date"`
}

// CreateProject godoc
// @Summary Create a new project
// @Tags projects
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body createProjectRequest true "Project payload"
// @Success 201 {object} map[string]interface{}
// @Router /api/v1/projects [post]
func (h *ProjectHandler) CreateProject(c *gin.Context) {
	userID, companyID, _, err := middleware.GetClaims(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	var req createProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := h.taskClient.CreateProject(c.Request.Context(), &taskv1.CreateProjectRequest{
		CompanyId:   companyID,
		CreatedBy:   userID,
		Name:        req.Name,
		Description: req.Description,
		StartDate:   req.StartDate,
		EndDate:     req.EndDate,
	})
	if err != nil {
		c.JSON(grpcToHTTPStatus(err), gin.H{"error": grpcMessage(err)})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"project": resp.Project})
}

// GetProject godoc
// @Summary Get a project by ID
// @Tags projects
// @Security BearerAuth
// @Produce json
// @Param id path string true "Project ID"
// @Success 200 {object} map[string]interface{}
// @Failure 401,404 {object} map[string]string
// @Router /api/v1/projects/{id} [get]
func (h *ProjectHandler) GetProject(c *gin.Context) {
	_, companyID, _, err := middleware.GetClaims(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	resp, err := h.taskClient.GetProject(c.Request.Context(), &taskv1.GetProjectRequest{
		CompanyId: companyID,
		Id:        c.Param("id"),
	})
	if err != nil {
		c.JSON(grpcToHTTPStatus(err), gin.H{"error": grpcMessage(err)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"project": resp.Project})
}

// ListProjects godoc
// @Summary List all projects in the company
// @Tags projects
// @Security BearerAuth
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(20)
// @Param status query string false "Filter by status"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]string
// @Router /api/v1/projects [get]
func (h *ProjectHandler) ListProjects(c *gin.Context) {
	_, companyID, _, err := middleware.GetClaims(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	resp, err := h.taskClient.ListProjects(c.Request.Context(), &taskv1.ListProjectsRequest{
		CompanyId: companyID,
		Status:    c.Query("status"),
		Page:      int32(page),
		PageSize:  int32(pageSize),
	})
	if err != nil {
		c.JSON(grpcToHTTPStatus(err), gin.H{"error": grpcMessage(err)})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"projects":  resp.Projects,
		"total":     resp.Total,
		"page":      resp.Page,
		"page_size": resp.PageSize,
	})
}

type updateProjectRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
	EndDate     string `json:"end_date"`
}

// UpdateProject godoc
// @Summary Update a project
// @Tags projects
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Project ID"
// @Param body body updateProjectRequest true "Update payload"
// @Success 200 {object} map[string]interface{}
// @Failure 400,401,403,404 {object} map[string]string
// @Router /api/v1/projects/{id} [put]
func (h *ProjectHandler) UpdateProject(c *gin.Context) {
	_, companyID, _, err := middleware.GetClaims(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	var req updateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := h.taskClient.UpdateProject(c.Request.Context(), &taskv1.UpdateProjectRequest{
		CompanyId:   companyID,
		Id:          c.Param("id"),
		Name:        req.Name,
		Description: req.Description,
		Status:      req.Status,
		EndDate:     req.EndDate,
	})
	if err != nil {
		c.JSON(grpcToHTTPStatus(err), gin.H{"error": grpcMessage(err)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"project": resp.Project})
}

// DeleteProject godoc
// @Summary Delete a project
// @Tags projects
// @Security BearerAuth
// @Param id path string true "Project ID"
// @Success 204
// @Failure 401,403,404 {object} map[string]string
// @Router /api/v1/projects/{id} [delete]
func (h *ProjectHandler) DeleteProject(c *gin.Context) {
	_, companyID, _, err := middleware.GetClaims(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	_, err = h.taskClient.DeleteProject(c.Request.Context(), &taskv1.DeleteProjectRequest{
		CompanyId: companyID,
		Id:        c.Param("id"),
	})
	if err != nil {
		c.JSON(grpcToHTTPStatus(err), gin.H{"error": grpcMessage(err)})
		return
	}
	c.Status(http.StatusNoContent)
}
