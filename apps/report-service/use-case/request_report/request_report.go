package request_report

import (
	"context"

	"github.com/hodynguyen/construct-flow/apps/report-service/domain"
	"github.com/hodynguyen/construct-flow/apps/report-service/entity/model"
)

type Input struct {
	CompanyID   string
	RequestedBy string
	JobType     string
	Params      string // JSON string
}

type Output struct {
	JobID string
}

type UseCase struct {
	repo      domain.ReportRepository
	publisher domain.EventPublisher
}

func New(repo domain.ReportRepository, publisher domain.EventPublisher) *UseCase {
	return &UseCase{repo: repo, publisher: publisher}
}

func (uc *UseCase) Execute(ctx context.Context, in Input) (*Output, error) {
	job := &model.ReportJob{
		CompanyID:   in.CompanyID,
		RequestedBy: in.RequestedBy,
		Type:        in.JobType,
		Params:      in.Params,
		Status:      "queued",
	}

	if err := uc.repo.Create(ctx, job); err != nil {
		return nil, err
	}

	if err := uc.publisher.PublishReportRequested(ctx, job.ID, job.CompanyID, job.Type, job.Params); err != nil {
		// Job already persisted; log the publish failure but don't fail the request.
		// A scheduler can re-trigger stuck queued jobs.
		_ = err
	}

	return &Output{JobID: job.ID}, nil
}
