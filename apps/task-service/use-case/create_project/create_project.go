package create_project

import (
	"context"
	"time"

	"github.com/hodynguyen/construct-flow/apps/task-service/common"
	"github.com/hodynguyen/construct-flow/apps/task-service/domain"
	"github.com/hodynguyen/construct-flow/apps/task-service/entity/dto"
	"github.com/hodynguyen/construct-flow/apps/task-service/entity/model"
)

type UseCase struct {
	projectRepo domain.ProjectRepository
}

func New(projectRepo domain.ProjectRepository) *UseCase {
	return &UseCase{projectRepo: projectRepo}
}

func (uc *UseCase) Execute(ctx context.Context, req dto.CreateProjectRequest) (*dto.ProjectResponse, error) {
	if req.Name == "" || req.CompanyID == "" || req.CreatedBy == "" {
		return nil, common.ErrInvalidInput
	}

	project := &model.Project{
		CompanyID:   req.CompanyID,
		Name:        req.Name,
		Description: req.Description,
		CreatedBy:   req.CreatedBy,
		Status:      "active",
	}

	if req.StartDate != "" {
		t, err := time.Parse("2006-01-02", req.StartDate)
		if err != nil {
			return nil, common.ErrInvalidInput
		}
		project.StartDate = &t
	}

	if req.EndDate != "" {
		t, err := time.Parse("2006-01-02", req.EndDate)
		if err != nil {
			return nil, common.ErrInvalidInput
		}
		project.EndDate = &t
	}

	if err := uc.projectRepo.Create(ctx, project); err != nil {
		return nil, err
	}

	return dto.ToProjectResponse(project), nil
}
