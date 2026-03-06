package get_download_url

import (
	"context"
	"fmt"
	"time"

	"github.com/hodynguyen/construct-flow/apps/file-service/common"
	"github.com/hodynguyen/construct-flow/apps/file-service/domain"
)

const downloadTTL = 15 * time.Minute

type Output struct {
	DownloadURL     string
	StorageTier     string
	RequiresRestore bool
}

type UseCase struct {
	repo    domain.FileRepository
	storage domain.StorageClient
}

func New(repo domain.FileRepository, storage domain.StorageClient) *UseCase {
	return &UseCase{repo: repo, storage: storage}
}

func (uc *UseCase) Execute(ctx context.Context, companyID, fileID string) (Output, error) {
	file, err := uc.repo.FindByID(ctx, companyID, fileID)
	if err != nil {
		return Output{}, err
	}
	if file.Status != "active" {
		return Output{}, common.ErrNotFound
	}

	// Glacier files require a restore request (can take hours) before download
	if file.StorageTier == "glacier" {
		return Output{
			StorageTier:     "glacier",
			RequiresRestore: true,
		}, common.ErrGlacierRestore
	}

	downloadURL, err := uc.storage.PresignGetURL(ctx, file.S3Bucket, file.S3Key, downloadTTL)
	if err != nil {
		return Output{}, fmt.Errorf("presigning download URL: %w", err)
	}

	return Output{
		DownloadURL:     downloadURL,
		StorageTier:     file.StorageTier,
		RequiresRestore: false,
	}, nil
}
