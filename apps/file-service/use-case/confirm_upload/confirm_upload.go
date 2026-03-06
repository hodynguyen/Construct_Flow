package confirm_upload

import (
	"context"
	"fmt"

	"github.com/hodynguyen/construct-flow/apps/file-service/common"
	"github.com/hodynguyen/construct-flow/apps/file-service/domain"
	"github.com/hodynguyen/construct-flow/apps/file-service/entity/model"
)

type UseCase struct {
	repo domain.FileRepository
}

func New(repo domain.FileRepository) *UseCase {
	return &UseCase{repo: repo}
}

func (uc *UseCase) Execute(ctx context.Context, companyID, fileID string, sizeBytes int64) (*model.File, error) {
	file, err := uc.repo.FindByID(ctx, companyID, fileID)
	if err != nil {
		return nil, err
	}
	if file.Status != "pending" {
		return nil, fmt.Errorf("%w: file already confirmed or deleted", common.ErrInvalidInput)
	}

	if err := uc.repo.UpdateStatus(ctx, fileID, "active", sizeBytes); err != nil {
		return nil, fmt.Errorf("confirming upload: %w", err)
	}

	file.Status = "active"
	file.SizeBytes = sizeBytes
	return file, nil
}
