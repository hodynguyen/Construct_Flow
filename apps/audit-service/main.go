package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"github.com/hodynguyen/construct-flow/apps/audit-service/api/grpc/controller"
	"github.com/hodynguyen/construct-flow/apps/audit-service/bootstrap"
	"github.com/hodynguyen/construct-flow/apps/audit-service/consumer"
	sqlrepo "github.com/hodynguyen/construct-flow/apps/audit-service/repository/sql"
	auditv1 "github.com/hodynguyen/construct-flow/gen/go/proto/audit_service/v1"
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

	auditRepo := sqlrepo.NewAuditRepository(db)

	grpcServer := bootstrap.NewGRPCServer()
	auditv1.RegisterAuditServiceServer(grpcServer, controller.NewAuditController(auditRepo))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go consumer.NewEventConsumer(mq.Channel, auditRepo, logger).Start(ctx)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		logger.Info("shutting down audit-service")
		cancel()
		grpcServer.GracefulStop()
	}()

	logger.Info("audit-service gRPC listening", zap.Int("port", cfg.GRPCPort))
	if err := bootstrap.StartGRPCServer(grpcServer, cfg.GRPCPort); err != nil {
		logger.Fatal("gRPC server error", zap.Error(err))
	}
}
