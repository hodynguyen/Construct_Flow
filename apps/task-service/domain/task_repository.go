package domain

import (
	"context"

	"github.com/hodynguyen/construct-flow/apps/task-service/entity/model"
)

type ListTaskFilter struct {
	Status     string
	AssignedTo string
	Priority   string
}

type TaskRepository interface {
	Create(ctx context.Context, task *model.Task) error
	FindByID(ctx context.Context, companyID, taskID string) (*model.Task, error)
	Update(ctx context.Context, task *model.Task) error
	Delete(ctx context.Context, companyID, taskID string) error
	ListByProject(ctx context.Context, companyID, projectID string, filter ListTaskFilter, page, pageSize int) ([]model.Task, int64, error)
}
