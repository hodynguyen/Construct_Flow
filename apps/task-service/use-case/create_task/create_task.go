package create_task

import (
	"context"
	"time"

	"github.com/hodynguyen/construct-flow/apps/task-service/common"
	"github.com/hodynguyen/construct-flow/apps/task-service/domain"
	"github.com/hodynguyen/construct-flow/apps/task-service/entity/dto"
	"github.com/hodynguyen/construct-flow/apps/task-service/entity/model"
)

type UseCase struct {
	taskRepo    domain.TaskRepository
	projectRepo domain.ProjectRepository
}

func New(taskRepo domain.TaskRepository, projectRepo domain.ProjectRepository) *UseCase {
	return &UseCase{taskRepo: taskRepo, projectRepo: projectRepo}
}

func (uc *UseCase) Execute(ctx context.Context, req dto.CreateTaskRequest) (*dto.TaskResponse, error) {
	if req.Title == "" || req.CompanyID == "" || req.ProjectID == "" || req.CreatedBy == "" {
		return nil, common.ErrInvalidInput
	}

	// Validate project exists and belongs to this company (tenant check)
	if _, err := uc.projectRepo.FindByID(ctx, req.CompanyID, req.ProjectID); err != nil {
		return nil, err
	}

	priority := req.Priority
	if priority == "" {
		priority = "medium"
	}

	task := &model.Task{
		ProjectID:   req.ProjectID,
		CompanyID:   req.CompanyID,
		Title:       req.Title,
		Description: req.Description,
		Priority:    priority,
		CreatedBy:   req.CreatedBy,
		Status:      "todo",
	}

	if req.AssignedTo != "" {
		task.AssignedTo = &req.AssignedTo
	}

	if req.DueDate != "" {
		t, err := time.Parse("2006-01-02", req.DueDate)
		if err != nil {
			return nil, common.ErrInvalidInput
		}
		task.DueDate = &t
	}

	if err := uc.taskRepo.Create(ctx, task); err != nil {
		return nil, err
	}

	return dto.ToTaskResponse(task), nil
}
