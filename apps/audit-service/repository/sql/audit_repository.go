package sql

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/hodynguyen/construct-flow/apps/audit-service/domain"
	"github.com/hodynguyen/construct-flow/apps/audit-service/entity/model"
)

type auditRepository struct {
	db *gorm.DB
}

func NewAuditRepository(db *gorm.DB) domain.AuditRepository {
	return &auditRepository{db: db}
}

func (r *auditRepository) Append(ctx context.Context, log *model.AuditLog) error {
	if log.ID == "" {
		log.ID = uuid.NewString()
	}
	// INSERT only — audit logs are immutable
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *auditRepository) Query(ctx context.Context, f domain.QueryFilter) ([]model.AuditLog, int64, error) {
	var logs []model.AuditLog
	var total int64

	q := r.db.WithContext(ctx).Where("company_id = ?", f.CompanyID)

	if f.Resource != "" {
		q = q.Where("resource = ?", f.Resource)
	}
	if f.ResourceID != "" {
		q = q.Where("resource_id = ?", f.ResourceID)
	}
	if f.UserID != "" {
		q = q.Where("user_id = ?", f.UserID)
	}
	if f.Action != "" {
		q = q.Where("action = ?", f.Action)
	}
	if f.From != nil {
		q = q.Where("occurred_at >= ?", f.From)
	}
	if f.To != nil {
		q = q.Where("occurred_at <= ?", f.To)
	}

	q.Model(&model.AuditLog{}).Count(&total)

	page := f.Page
	if page < 1 {
		page = 1
	}
	pageSize := f.PageSize
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}

	err := q.Order("occurred_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&logs).Error

	return logs, total, err
}
