package sql

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/hodynguyen/construct-flow/apps/report-service/common"
	"github.com/hodynguyen/construct-flow/apps/report-service/domain"
	"github.com/hodynguyen/construct-flow/apps/report-service/entity/model"
)

type reportRepository struct {
	db *gorm.DB
}

func NewReportRepository(db *gorm.DB) domain.ReportRepository {
	return &reportRepository{db: db}
}

func (r *reportRepository) Create(ctx context.Context, job *model.ReportJob) error {
	return r.db.WithContext(ctx).Create(job).Error
}

func (r *reportRepository) FindByID(ctx context.Context, companyID, jobID string) (*model.ReportJob, error) {
	var job model.ReportJob
	err := r.db.WithContext(ctx).
		Where("id = ? AND company_id = ?", jobID, companyID).
		First(&job).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, common.ErrNotFound
	}
	return &job, err
}

func (r *reportRepository) List(ctx context.Context, companyID, requestedBy, status string, page, pageSize int) ([]model.ReportJob, int64, error) {
	var jobs []model.ReportJob
	var total int64

	q := r.db.WithContext(ctx).Where("company_id = ?", companyID)
	if requestedBy != "" {
		q = q.Where("requested_by = ?", requestedBy)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}

	q.Model(&model.ReportJob{}).Count(&total)
	err := q.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&jobs).Error
	return jobs, total, err
}

func (r *reportRepository) UpdateStatus(ctx context.Context, jobID, status, s3Key, downloadURL, errMsg string) error {
	updates := map[string]interface{}{
		"status":       status,
		"completed_at": time.Now(),
	}
	if s3Key != "" {
		updates["s3_key"] = s3Key
	}
	if downloadURL != "" {
		updates["download_url"] = downloadURL
	}
	if errMsg != "" {
		updates["error_msg"] = errMsg
	}
	return r.db.WithContext(ctx).Model(&model.ReportJob{}).
		Where("id = ?", jobID).
		Updates(updates).Error
}
