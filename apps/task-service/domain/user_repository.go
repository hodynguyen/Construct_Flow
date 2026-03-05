package domain

import (
	"context"

	"github.com/hodynguyen/construct-flow/apps/task-service/entity/model"
)

// UserRepository is a read-only interface in task-service.
// Used to validate that an assignee belongs to the same company before assigning a task.
type UserRepository interface {
	FindByID(ctx context.Context, companyID, userID string) (*model.User, error)
}
