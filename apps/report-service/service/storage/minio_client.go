package storage

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/hodynguyen/construct-flow/apps/report-service/bootstrap"
)

type MinIOClient struct {
	client     *minio.Client
	bucketName string
}

func NewMinIOClient(cfg *bootstrap.Config) (*MinIOClient, error) {
	client, err := minio.New(cfg.S3Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.S3AccessKeyID, cfg.S3SecretAccessKey, ""),
		Secure: cfg.S3UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("creating minio client: %w", err)
	}
	return &MinIOClient{client: client, bucketName: cfg.S3BucketName}, nil
}

// UploadJSON uploads a JSON payload and returns the object key.
func (m *MinIOClient) UploadJSON(ctx context.Context, key string, data []byte) error {
	_, err := m.client.PutObject(ctx, m.bucketName, key, bytes.NewReader(data), int64(len(data)),
		minio.PutObjectOptions{ContentType: "application/json"},
	)
	return err
}

// PresignGetURL returns a time-limited download URL.
func (m *MinIOClient) PresignGetURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	u, err := m.client.PresignedGetObject(ctx, m.bucketName, key, ttl, nil)
	if err != nil {
		return "", fmt.Errorf("presigning get url: %w", err)
	}
	return u.String(), nil
}
