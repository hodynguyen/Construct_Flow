package domain

import (
	"context"

	"github.com/hodynguyen/construct-flow/apps/user-service/entity/model"
)

type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	FindByID(ctx context.Context, companyID, userID string) (*model.User, error)
	FindByEmail(ctx context.Context, email string) (*model.User, error)
	ExistsByEmail(ctx context.Context, email, companyID string) (bool, error)
}
