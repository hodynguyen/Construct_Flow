package presign_upload

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/hodynguyen/construct-flow/apps/file-service/common"
	"github.com/hodynguyen/construct-flow/apps/file-service/domain"
	"github.com/hodynguyen/construct-flow/apps/file-service/entity/model"
)

const presignTTL = 15 * time.Minute

type Input struct {
	CompanyID  string
	UploadedBy string
	Name       string
	MimeType   string
	SizeBytes  int64
	ProjectID  string // optional
	TaskID     string // optional
}

type Output struct {
	FileID    string
	UploadURL string
	S3Key     string
}

type UseCase struct {
	repo    domain.FileRepository
	storage domain.StorageClient
	bucket  string
}

func New(repo domain.FileRepository, storage domain.StorageClient, bucket string) *UseCase {
	return &UseCase{repo: repo, storage: storage, bucket: bucket}
}

func (uc *UseCase) Execute(ctx context.Context, in Input) (Output, error) {
	if in.CompanyID == "" || in.UploadedBy == "" || in.Name == "" {
		return Output{}, common.ErrInvalidInput
	}

	fileID := uuid.NewString()
	s3Key := fmt.Sprintf("%s/%s/%s", in.CompanyID, fileID, in.Name)

	uploadURL, err := uc.storage.PresignPutURL(ctx, uc.bucket, s3Key, in.MimeType, presignTTL)
	if err != nil {
		return Output{}, fmt.Errorf("presigning upload URL: %w", err)
	}

	file := &model.File{
		ID:         fileID,
		CompanyID:  in.CompanyID,
		UploadedBy: in.UploadedBy,
		Name:       in.Name,
		S3Key:      s3Key,
		S3Bucket:   uc.bucket,
		MimeType:   in.MimeType,
		SizeBytes:  in.SizeBytes,
		Status:     "pending",
	}
	if in.ProjectID != "" {
		file.ProjectID = &in.ProjectID
	}
	if in.TaskID != "" {
		file.TaskID = &in.TaskID
	}

	if err := uc.repo.Create(ctx, file); err != nil {
		return Output{}, fmt.Errorf("creating file record: %w", err)
	}

	return Output{FileID: fileID, UploadURL: uploadURL, S3Key: s3Key}, nil
}
