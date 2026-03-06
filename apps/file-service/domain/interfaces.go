package domain

import (
	"context"
	"time"

	"github.com/hodynguyen/construct-flow/apps/file-service/entity/model"
)

// FileRepository defines DB operations for files.
type FileRepository interface {
	Create(ctx context.Context, file *model.File) error
	FindByID(ctx context.Context, companyID, fileID string) (*model.File, error)
	ListByResource(ctx context.Context, companyID, projectID, taskID string, page, pageSize int) ([]model.File, int64, error)
	UpdateStatus(ctx context.Context, fileID, status string, sizeBytes int64) error
	UpdateStorageTier(ctx context.Context, fileID, tier string) error
	SoftDelete(ctx context.Context, companyID, fileID string) error
	// ListForLifecycleMigration returns active files older than cutoff for a given current tier.
	ListForLifecycleMigration(ctx context.Context, companyID string, currentTier string, olderThan time.Time) ([]model.File, error)
}

// StorageClient abstracts S3/MinIO operations.
type StorageClient interface {
	PresignPutURL(ctx context.Context, bucket, key, mimeType string, ttl time.Duration) (string, error)
	PresignGetURL(ctx context.Context, bucket, key string, ttl time.Duration) (string, error)
	DeleteObject(ctx context.Context, bucket, key string) error
	// CopyWithStorageClass copies an object to a new storage class (simulates S3 tier migration).
	CopyWithStorageClass(ctx context.Context, bucket, key, targetClass string) error
}
