package sql

import (
	"context"
	"errors"

	"github.com/hodynguyen/construct-flow/apps/task-service/common"
	"github.com/hodynguyen/construct-flow/apps/task-service/domain"
	"github.com/hodynguyen/construct-flow/apps/task-service/entity/model"
	"gorm.io/gorm"
)

type taskRepository struct {
	db *gorm.DB
}

func NewTaskRepository(db *gorm.DB) domain.TaskRepository {
	return &taskRepository{db: db}
}

func (r *taskRepository) Create(ctx context.Context, task *model.Task) error {
	return r.db.WithContext(ctx).Create(task).Error
}

func (r *taskRepository) FindByID(ctx context.Context, companyID, taskID string) (*model.Task, error) {
	var task model.Task
	err := r.db.WithContext(ctx).
		Where("id = ? AND company_id = ?", taskID, companyID).
		First(&task).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, common.ErrNotFound
	}
	return &task, err
}

func (r *taskRepository) Update(ctx context.Context, task *model.Task) error {
	return r.db.WithContext(ctx).Save(task).Error
}

func (r *taskRepository) Delete(ctx context.Context, companyID, taskID string) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND company_id = ?", taskID, companyID).
		Delete(&model.Task{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return common.ErrNotFound
	}
	return nil
}

func (r *taskRepository) ListByProject(ctx context.Context, companyID, projectID string, filter domain.ListTaskFilter, page, pageSize int) ([]model.Task, int64, error) {
	q := r.db.WithContext(ctx).
		Where("company_id = ? AND project_id = ?", companyID, projectID)

	if filter.Status != "" {
		q = q.Where("status = ?", filter.Status)
	}
	if filter.AssignedTo != "" {
		q = q.Where("assigned_to = ?", filter.AssignedTo)
	}
	if filter.Priority != "" {
		q = q.Where("priority = ?", filter.Priority)
	}

	var total int64
	if err := q.Model(&model.Task{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var tasks []model.Task
	offset := (page - 1) * pageSize
	err := q.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&tasks).Error
	return tasks, total, err
}
