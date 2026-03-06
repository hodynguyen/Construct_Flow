package sql

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/hodynguyen/construct-flow/apps/file-service/common"
	"github.com/hodynguyen/construct-flow/apps/file-service/domain"
	"github.com/hodynguyen/construct-flow/apps/file-service/entity/model"
)

type fileRepository struct {
	db *gorm.DB
}

func NewFileRepository(db *gorm.DB) domain.FileRepository {
	return &fileRepository{db: db}
}

func (r *fileRepository) Create(ctx context.Context, file *model.File) error {
	return r.db.WithContext(ctx).Create(file).Error
}

func (r *fileRepository) FindByID(ctx context.Context, companyID, fileID string) (*model.File, error) {
	var file model.File
	err := r.db.WithContext(ctx).
		Where("id = ? AND company_id = ?", fileID, companyID).
		First(&file).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, common.ErrNotFound
	}
	return &file, err
}

func (r *fileRepository) ListByResource(ctx context.Context, companyID, projectID, taskID string, page, pageSize int) ([]model.File, int64, error) {
	var files []model.File
	var total int64

	q := r.db.WithContext(ctx).Where("company_id = ? AND status = 'active'", companyID)
	if projectID != "" {
		q = q.Where("project_id = ?", projectID)
	}
	if taskID != "" {
		q = q.Where("task_id = ?", taskID)
	}

	q.Model(&model.File{}).Count(&total)
	err := q.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&files).Error
	return files, total, err
}

func (r *fileRepository) UpdateStatus(ctx context.Context, fileID, status string, sizeBytes int64) error {
	updates := map[string]interface{}{"status": status}
	if sizeBytes > 0 {
		updates["size_bytes"] = sizeBytes
	}
	return r.db.WithContext(ctx).Model(&model.File{}).
		Where("id = ?", fileID).
		Updates(updates).Error
}

func (r *fileRepository) UpdateStorageTier(ctx context.Context, fileID, tier string) error {
	return r.db.WithContext(ctx).Model(&model.File{}).
		Where("id = ?", fileID).
		Update("storage_tier", tier).Error
}

func (r *fileRepository) SoftDelete(ctx context.Context, companyID, fileID string) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND company_id = ?", fileID, companyID).
		Delete(&model.File{})
	if result.RowsAffected == 0 {
		return common.ErrNotFound
	}
	return result.Error
}

func (r *fileRepository) ListForLifecycleMigration(ctx context.Context, companyID string, currentTier string, olderThan time.Time) ([]model.File, error) {
	var files []model.File
	err := r.db.WithContext(ctx).
		Where("company_id = ? AND storage_tier = ? AND status = 'active' AND created_at < ?",
			companyID, currentTier, olderThan).
		Find(&files).Error
	return files, err
}
