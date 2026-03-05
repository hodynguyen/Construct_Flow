package sql

import (
	"context"
	"errors"

	"github.com/hodynguyen/construct-flow/apps/task-service/common"
	"github.com/hodynguyen/construct-flow/apps/task-service/domain"
	"github.com/hodynguyen/construct-flow/apps/task-service/entity/model"
	"gorm.io/gorm"
)

type projectRepository struct {
	db *gorm.DB
}

func NewProjectRepository(db *gorm.DB) domain.ProjectRepository {
	return &projectRepository{db: db}
}

func (r *projectRepository) Create(ctx context.Context, project *model.Project) error {
	return r.db.WithContext(ctx).Create(project).Error
}

func (r *projectRepository) FindByID(ctx context.Context, companyID, projectID string) (*model.Project, error) {
	var project model.Project
	err := r.db.WithContext(ctx).
		Where("id = ? AND company_id = ?", projectID, companyID).
		First(&project).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, common.ErrNotFound
	}
	return &project, err
}

func (r *projectRepository) Update(ctx context.Context, project *model.Project) error {
	return r.db.WithContext(ctx).Save(project).Error
}

func (r *projectRepository) Delete(ctx context.Context, companyID, projectID string) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND company_id = ?", projectID, companyID).
		Delete(&model.Project{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return common.ErrNotFound
	}
	return nil
}

func (r *projectRepository) ListByCompany(ctx context.Context, companyID string, filter domain.ListProjectFilter, page, pageSize int) ([]model.Project, int64, error) {
	q := r.db.WithContext(ctx).Where("company_id = ?", companyID)

	if filter.Status != "" {
		q = q.Where("status = ?", filter.Status)
	}

	var total int64
	if err := q.Model(&model.Project{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var projects []model.Project
	offset := (page - 1) * pageSize
	err := q.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&projects).Error
	return projects, total, err
}
