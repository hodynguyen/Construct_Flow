package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"github.com/hodynguyen/construct-flow/apps/scheduler-service/api/grpc/controller"
	"github.com/hodynguyen/construct-flow/apps/scheduler-service/bootstrap"
	"github.com/hodynguyen/construct-flow/apps/scheduler-service/job"
	schedulerv1 "github.com/hodynguyen/construct-flow/gen/go/proto/scheduler_service/v1"
)

func main() {
	logger, _ := zap.NewProduction()
	defer func() { _ = logger.Sync() }()

	cfg, err := bootstrap.LoadConfig()
	if err != nil {
		log.Fatalf("loading config: %v", err)
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

	pub := job.NewPublisher(mq.Channel)

	// Register cron jobs with distributed locks
	scheduler := bootstrap.NewScheduler(redisClient, logger)

	jobs := []struct {
		name     string
		schedule string
		fn       func(ctx context.Context)
	}{
		{"deadline_checker", "*/1 * * * *", job.DeadlineChecker(pub, logger)},   // every 1 min
		{"overdue_checker", "*/5 * * * *", job.OverdueChecker(pub, logger)},     // every 5 min
		{"weekly_report", "0 8 * * 1", job.WeeklyReport(pub, logger)},           // Mon 8am
		{"file_lifecycle", "0 2 * * *", job.FileLifecycle(pub, logger)},         // daily 2am
	}

	for _, j := range jobs {
		if err := scheduler.AddJob(j.name, j.schedule, j.fn); err != nil {
			log.Fatalf("registering job %s: %v", j.name, err)
		}
	}

	scheduler.Start()
	defer scheduler.Stop()

	// gRPC for admin ListJobs
	grpcServer := bootstrap.NewGRPCServer()
	schedulerv1.RegisterSchedulerServiceServer(grpcServer, controller.NewSchedulerController(scheduler))

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		logger.Info("shutting down scheduler-service")
		grpcServer.GracefulStop()
	}()

	logger.Info("scheduler-service gRPC listening", zap.Int("port", cfg.GRPCPort))
	if err := bootstrap.StartGRPCServer(grpcServer, cfg.GRPCPort); err != nil {
		logger.Fatal("gRPC server error", zap.Error(err))
	}
}
