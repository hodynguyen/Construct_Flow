package migrate_storage

import (
	"context"
	"fmt"
	"time"

	"github.com/hodynguyen/construct-flow/apps/file-service/domain"
)

// StorageTierPolicy defines how long files stay in each tier before migration.
var StorageTierPolicy = map[string]struct {
	NextTier string
	AfterAge time.Duration
}{
	"standard":    {NextTier: "standard_ia", AfterAge: 180 * 24 * time.Hour}, // 6 months
	"standard_ia": {NextTier: "glacier", AfterAge: 2 * 365 * 24 * time.Hour}, // 2 years
}

type UseCase struct {
	repo    domain.FileRepository
	storage domain.StorageClient
}

func New(repo domain.FileRepository, storage domain.StorageClient) *UseCase {
	return &UseCase{repo: repo, storage: storage}
}

// Execute migrates files in companyID that have exceeded the tier age threshold.
// Called by scheduler-service periodically.
func (uc *UseCase) Execute(ctx context.Context, companyID, targetTier string) (int, error) {
	policy, ok := StorageTierPolicy[targetTier]
	_ = policy
	if !ok {
		// Determine source tier: standard_ia migrates from standard, glacier from standard_ia
		currentTier := "standard"
		if targetTier == "glacier" {
			currentTier = "standard_ia"
		}
		policy = StorageTierPolicy[currentTier]
	}

	// Determine current tier from target
	currentTier := "standard"
	if targetTier == "glacier" {
		currentTier = "standard_ia"
	}

	cutoff := time.Now().Add(-StorageTierPolicy[currentTier].AfterAge)
	files, err := uc.repo.ListForLifecycleMigration(ctx, companyID, currentTier, cutoff)
	if err != nil {
		return 0, fmt.Errorf("listing files for migration: %w", err)
	}

	migrated := 0
	for _, file := range files {
		// In AWS: CopyObject with new StorageClass. In MinIO: no-op (tier tracked in DB only).
		if err := uc.storage.CopyWithStorageClass(ctx, file.S3Bucket, file.S3Key, targetTier); err != nil {
			continue // best-effort: skip failed files
		}
		if err := uc.repo.UpdateStorageTier(ctx, file.ID, targetTier); err != nil {
			continue
		}
		migrated++
	}
	return migrated, nil
}
