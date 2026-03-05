package domain

import (
	"context"

	"github.com/hodynguyen/construct-flow/apps/task-service/entity/model"
)

type ListProjectFilter struct {
	Status string
}

type ProjectRepository interface {
	Create(ctx context.Context, project *model.Project) error
	FindByID(ctx context.Context, companyID, projectID string) (*model.Project, error)
	Update(ctx context.Context, project *model.Project) error
	Delete(ctx context.Context, companyID, projectID string) error
	ListByCompany(ctx context.Context, companyID string, filter ListProjectFilter, page, pageSize int) ([]model.Project, int64, error)
}
