package main

import (
	"log"

	"github.com/hodynguyen/construct-flow/apps/task-service/api/grpc/controller"
	"github.com/hodynguyen/construct-flow/apps/task-service/bootstrap"
	sqlrepo "github.com/hodynguyen/construct-flow/apps/task-service/repository/sql"
	"github.com/hodynguyen/construct-flow/apps/task-service/service"
	"github.com/hodynguyen/construct-flow/apps/task-service/use-case/assign_task"
	"github.com/hodynguyen/construct-flow/apps/task-service/use-case/create_project"
	"github.com/hodynguyen/construct-flow/apps/task-service/use-case/create_task"
	"github.com/hodynguyen/construct-flow/apps/task-service/use-case/update_task_status"
	taskv1 "github.com/hodynguyen/construct-flow/gen/go/proto/task_service/v1"
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

	redisClient, err := bootstrap.NewRedisClient(cfg)
	if err != nil {
		log.Fatalf("connecting to redis: %v", err)
	}

	mq, err := bootstrap.NewRabbitMQ(cfg)
	if err != nil {
		log.Fatalf("connecting to rabbitmq: %v", err)
	}
	defer mq.Close()

	// Repositories
	taskRepo := sqlrepo.NewTaskRepository(db)
	projectRepo := sqlrepo.NewProjectRepository(db)
	userRepo := sqlrepo.NewUserRepository(db)

	// Services
	lockClient := service.NewRedisLockClient(redisClient)
	publisher := service.NewRabbitMQPublisher(mq.Channel)

	// Use cases
	createProjectUC := create_project.New(projectRepo)
	createTaskUC := create_task.New(taskRepo, projectRepo)
	assignTaskUC := assign_task.New(taskRepo, userRepo, publisher, lockClient)
	updateStatusUC := update_task_status.New(taskRepo, publisher)

	// gRPC server
	grpcServer := bootstrap.NewGRPCServer()
	taskv1.RegisterTaskServiceServer(grpcServer, controller.NewTaskController(
		projectRepo, taskRepo,
		createProjectUC, createTaskUC, assignTaskUC, updateStatusUC,
	))

	log.Printf("task-service gRPC listening on :%d", cfg.GRPCPort)
	if err := bootstrap.StartGRPCServer(grpcServer, cfg.GRPCPort); err != nil {
		log.Fatalf("starting gRPC server: %v", err)
	}
}
