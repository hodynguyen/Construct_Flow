package sql

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/hodynguyen/construct-flow/apps/user-service/common"
	"github.com/hodynguyen/construct-flow/apps/user-service/entity/model"
)

type userRepository struct {
	db *gorm.DB
}

// NewUserRepository returns a GORM-backed UserRepository.
func NewUserRepository(db *gorm.DB) *userRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(ctx context.Context, user *model.User) error {
	if err := r.db.WithContext(ctx).Create(user).Error; err != nil {
		return err
	}
	return nil
}

func (r *userRepository) FindByID(ctx context.Context, companyID, userID string) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).
		Where("company_id = ? AND id = ?", companyID, userID).
		First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, common.ErrNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).
		Where("email = ?", email).
		First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, common.ErrNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) ExistsByEmail(ctx context.Context, email, companyID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.User{}).
		Where("email = ? AND company_id = ?", email, companyID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
