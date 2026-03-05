package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"github.com/hodynguyen/construct-flow/apps/notification-service/api/grpc/controller"
	"github.com/hodynguyen/construct-flow/apps/notification-service/bootstrap"
	"github.com/hodynguyen/construct-flow/apps/notification-service/consumer"
	sqlrepo "github.com/hodynguyen/construct-flow/apps/notification-service/repository/sql"
	"github.com/hodynguyen/construct-flow/apps/notification-service/use-case/create_notification"
	"github.com/hodynguyen/construct-flow/apps/notification-service/use-case/mark_read"
	notifv1 "github.com/hodynguyen/construct-flow/gen/go/proto/notification_service/v1"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	cfg, err := bootstrap.LoadConfig()
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	db, err := bootstrap.NewDatabase(cfg)
	if err != nil {
		log.Fatalf("connecting to database: %v", err)
	}

	redisClient, err := bootstrap.NewRedisClient(cfg)
	if err != nil {
		log.Fatalf("connecting to redis: %v", err)
	}

	mq, err := bootstrap.NewRabbitMQ(cfg)
	if err != nil {
		log.Fatalf("connecting to rabbitmq: %v", err)
	}
	defer mq.Close()

	// Repository + use cases
	notifRepo := sqlrepo.NewNotificationRepository(db)
	createNotifUC := create_notification.New(notifRepo, redisClient)
	markReadUC := mark_read.New(notifRepo)

	// gRPC server
	grpcServer := bootstrap.NewGRPCServer()
	notifv1.RegisterNotificationServiceServer(grpcServer,
		controller.NewNotificationController(notifRepo, markReadUC))

	// Context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start RabbitMQ consumers in goroutines
	go consumer.NewTaskAssignedConsumer(mq.Channel, createNotifUC, logger).Start(ctx)
	go consumer.NewTaskStatusChangedConsumer(mq.Channel, createNotifUC, logger).Start(ctx)

	// Handle OS signals for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		logger.Info("shutting down notification-service")
		cancel()
		grpcServer.GracefulStop()
	}()

	logger.Info("notification-service gRPC listening", zap.Int("port", cfg.GRPCPort))
	if err := bootstrap.StartGRPCServer(grpcServer, cfg.GRPCPort); err != nil {
		logger.Fatal("gRPC server error", zap.Error(err))
	}
}
