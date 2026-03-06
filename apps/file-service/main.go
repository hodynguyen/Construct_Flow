package main

import (
	"log"

	filev1 "github.com/hodynguyen/construct-flow/gen/go/proto/file_service/v1"

	"github.com/hodynguyen/construct-flow/apps/file-service/api/grpc/controller"
	"github.com/hodynguyen/construct-flow/apps/file-service/bootstrap"
	sqlrepo "github.com/hodynguyen/construct-flow/apps/file-service/repository/sql"
	"github.com/hodynguyen/construct-flow/apps/file-service/service/storage"
	"github.com/hodynguyen/construct-flow/apps/file-service/use-case/confirm_upload"
	"github.com/hodynguyen/construct-flow/apps/file-service/use-case/get_download_url"
	"github.com/hodynguyen/construct-flow/apps/file-service/use-case/migrate_storage"
	"github.com/hodynguyen/construct-flow/apps/file-service/use-case/presign_upload"
)

func main() {
	cfg, err := bootstrap.LoadConfig()
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	db, err := bootstrap.NewDatabase(cfg)
	if err != nil {
		log.Fatalf("connecting to database: %v", err)
	}

	storageClient, err := storage.NewMinIOClient(
		cfg.S3Endpoint, cfg.S3AccessKeyID, cfg.S3SecretAccessKey,
		cfg.S3BucketName, cfg.S3UseSSL,
	)
	if err != nil {
		log.Fatalf("connecting to MinIO: %v", err)
	}

	fileRepo := sqlrepo.NewFileRepository(db)

	presignUC := presign_upload.New(fileRepo, storageClient, cfg.S3BucketName)
	confirmUC := confirm_upload.New(fileRepo)
	downloadUC := get_download_url.New(fileRepo, storageClient)
	migrateUC := migrate_storage.New(fileRepo, storageClient)

	grpcServer := bootstrap.NewGRPCServer()
	filev1.RegisterFileServiceServer(grpcServer,
		controller.NewFileController(fileRepo, presignUC, confirmUC, downloadUC, migrateUC))

	log.Printf("file-service gRPC listening on port %d", cfg.GRPCPort)
	if err := bootstrap.StartGRPCServer(grpcServer, cfg.GRPCPort); err != nil {
		log.Fatalf("gRPC server error: %v", err)
	}
}
