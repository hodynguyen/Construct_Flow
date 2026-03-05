package main

import (
	"log"

	"github.com/hodynguyen/construct-flow/apps/task-service/bootstrap"
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
	_ = db

	redisClient, err := bootstrap.NewRedisClient(cfg)
	if err != nil {
		log.Fatalf("connecting to redis: %v", err)
	}
	_ = redisClient

	mq, err := bootstrap.NewRabbitMQ(cfg)
	if err != nil {
		log.Fatalf("connecting to rabbitmq: %v", err)
	}
	defer mq.Close()

	grpcServer := bootstrap.NewGRPCServer()

	log.Printf("task-service gRPC listening on :%d", cfg.GRPCPort)
	if err := bootstrap.StartGRPCServer(grpcServer, cfg.GRPCPort); err != nil {
		log.Fatalf("starting gRPC server: %v", err)
	}
}
