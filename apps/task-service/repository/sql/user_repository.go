package sql

import (
	"context"
	"errors"

	"github.com/hodynguyen/construct-flow/apps/task-service/common"
	"github.com/hodynguyen/construct-flow/apps/task-service/domain"
	"github.com/hodynguyen/construct-flow/apps/task-service/entity/model"
	"gorm.io/gorm"
)

// userRepository is READ-ONLY in task-service.
// It validates that an assignee belongs to the same company before task assignment.
type userRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) domain.UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) FindByID(ctx context.Context, companyID, userID string) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).
		Where("id = ? AND company_id = ?", userID, companyID).
		First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, common.ErrUserNotFound
	}
	return &user, err
}
