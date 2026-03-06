package domain

import (
	"context"
	"time"

	"github.com/hodynguyen/construct-flow/apps/audit-service/entity/model"
)

type AuditRepository interface {
	Append(ctx context.Context, log *model.AuditLog) error
	Query(ctx context.Context, q QueryFilter) ([]model.AuditLog, int64, error)
}

type QueryFilter struct {
	CompanyID  string
	Resource   string
	ResourceID string
	UserID     string
	Action     string
	From       *time.Time
	To         *time.Time
	Page       int
	PageSize   int
}
