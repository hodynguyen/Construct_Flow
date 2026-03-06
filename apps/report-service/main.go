package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"github.com/hodynguyen/construct-flow/apps/report-service/api/grpc/controller"
	"github.com/hodynguyen/construct-flow/apps/report-service/bootstrap"
	"github.com/hodynguyen/construct-flow/apps/report-service/consumer"
	sqlrepo "github.com/hodynguyen/construct-flow/apps/report-service/repository/sql"
	"github.com/hodynguyen/construct-flow/apps/report-service/service"
	"github.com/hodynguyen/construct-flow/apps/report-service/service/storage"
	"github.com/hodynguyen/construct-flow/apps/report-service/use-case/get_report_status"
	"github.com/hodynguyen/construct-flow/apps/report-service/use-case/request_report"
	reportv1 "github.com/hodynguyen/construct-flow/gen/go/proto/report_service/v1"
)

func main() {
	logger, _ := zap.NewProduction()
	defer func() { _ = logger.Sync() }()

	cfg, err := bootstrap.LoadConfig()
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	db, err := bootstrap.NewDatabase(cfg)
	if err != nil {
		log.Fatalf("connecting to database: %v", err)
	}

	mq, err := bootstrap.NewRabbitMQ(cfg)
	if err != nil {
		log.Fatalf("connecting to rabbitmq: %v", err)
	}
	defer mq.Close()

	minioClient, err := storage.NewMinIOClient(cfg)
	if err != nil {
		log.Fatalf("connecting to minio: %v", err)
	}

	// Infrastructure
	reportRepo := sqlrepo.NewReportRepository(db)
	publisher := service.NewReportEventPublisher(mq.Channel)

	// Use cases
	requestReportUC := request_report.New(reportRepo, publisher)
	getStatusUC := get_report_status.New(reportRepo)

	// gRPC server
	grpcServer := bootstrap.NewGRPCServer()
	reportv1.RegisterReportServiceServer(grpcServer,
		controller.NewReportController(requestReportUC, getStatusUC, reportRepo))

	// Context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start consumer
	go consumer.NewReportRequestedConsumer(mq.Channel, reportRepo, minioClient, publisher, logger).Start(ctx)

	// Signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		logger.Info("shutting down report-service")
		cancel()
		grpcServer.GracefulStop()
	}()

	logger.Info("report-service gRPC listening", zap.Int("port", cfg.GRPCPort))
	if err := bootstrap.StartGRPCServer(grpcServer, cfg.GRPCPort); err != nil {
		logger.Fatal("gRPC server error", zap.Error(err))
	}
}
