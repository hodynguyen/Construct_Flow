package storage

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/hodynguyen/construct-flow/apps/file-service/domain"
)

type minioClient struct {
	client *minio.Client
	bucket string
}

// NewMinIOClient creates a StorageClient backed by MinIO (S3-compatible).
func NewMinIOClient(endpoint, accessKey, secretKey, bucket string, useSSL bool) (domain.StorageClient, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("creating minio client: %w", err)
	}
	return &minioClient{client: client, bucket: bucket}, nil
}

func (m *minioClient) PresignPutURL(ctx context.Context, _, key, _ string, ttl time.Duration) (string, error) {
	u, err := m.client.PresignedPutObject(ctx, m.bucket, key, ttl)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func (m *minioClient) PresignGetURL(ctx context.Context, _, key string, ttl time.Duration) (string, error) {
	reqParams := make(url.Values)
	u, err := m.client.PresignedGetObject(ctx, m.bucket, key, ttl, reqParams)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func (m *minioClient) DeleteObject(ctx context.Context, _, key string) error {
	return m.client.RemoveObject(ctx, m.bucket, key, minio.RemoveObjectOptions{})
}

// CopyWithStorageClass simulates S3 storage class migration.
// MinIO doesn't support storage classes — in production this calls S3 CopyObject with StorageClass header.
// Demo: just logs the migration intent (tier metadata updated in DB by use-case).
func (m *minioClient) CopyWithStorageClass(_ context.Context, _, key, targetClass string) error {
	// In AWS S3: s3.CopyObject with StorageClass = targetClass
	// In MinIO (dev): no-op — tier tracked in DB only
	_ = key
	_ = targetClass
	return nil
}
