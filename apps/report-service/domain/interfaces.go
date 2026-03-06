package domain

import (
	"context"

	"github.com/hodynguyen/construct-flow/apps/report-service/entity/model"
)

type ReportRepository interface {
	Create(ctx context.Context, job *model.ReportJob) error
	FindByID(ctx context.Context, companyID, jobID string) (*model.ReportJob, error)
	List(ctx context.Context, companyID, requestedBy, status string, page, pageSize int) ([]model.ReportJob, int64, error)
	UpdateStatus(ctx context.Context, jobID, status, s3Key, downloadURL, errMsg string) error
}

// EventPublisher publishes report lifecycle events to RabbitMQ.
type EventPublisher interface {
	PublishReportRequested(ctx context.Context, jobID, companyID, jobType, params string) error
	PublishReportCompleted(ctx context.Context, jobID, companyID, requestedBy, downloadURL string) error
}
