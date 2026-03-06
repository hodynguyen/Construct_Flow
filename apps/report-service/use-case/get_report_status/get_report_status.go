package get_report_status

import (
	"context"

	"github.com/hodynguyen/construct-flow/apps/report-service/domain"
	"github.com/hodynguyen/construct-flow/apps/report-service/entity/model"
)

type UseCase struct {
	repo domain.ReportRepository
}

func New(repo domain.ReportRepository) *UseCase {
	return &UseCase{repo: repo}
}

func (uc *UseCase) Execute(ctx context.Context, companyID, jobID string) (*model.ReportJob, error) {
	return uc.repo.FindByID(ctx, companyID, jobID)
}
