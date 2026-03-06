package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"github.com/hodynguyen/construct-flow/apps/search-service/api/grpc/controller"
	"github.com/hodynguyen/construct-flow/apps/search-service/bootstrap"
	"github.com/hodynguyen/construct-flow/apps/search-service/consumer"
	"github.com/hodynguyen/construct-flow/apps/search-service/service/elastic"
	"github.com/hodynguyen/construct-flow/apps/search-service/use-case/search"
	searchv1 "github.com/hodynguyen/construct-flow/gen/go/proto/search_service/v1"
)

func main() {
	logger, _ := zap.NewProduction()
	defer func() { _ = logger.Sync() }()

	cfg, err := bootstrap.LoadConfig()
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	mq, err := bootstrap.NewRabbitMQ(cfg)
	if err != nil {
		log.Fatalf("connecting to rabbitmq: %v", err)
	}
	defer mq.Close()

	esClient := elastic.NewClient(cfg.ElasticsearchURL)
	searchUC := search.New(esClient)

	grpcServer := bootstrap.NewGRPCServer()
	searchv1.RegisterSearchServiceServer(grpcServer, controller.NewSearchController(searchUC))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go consumer.NewIndexerConsumer(mq.Channel, esClient, logger).Start(ctx)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		logger.Info("shutting down search-service")
		cancel()
		grpcServer.GracefulStop()
	}()

	logger.Info("search-service gRPC listening", zap.Int("port", cfg.GRPCPort))
	if err := bootstrap.StartGRPCServer(grpcServer, cfg.GRPCPort); err != nil {
		logger.Fatal("gRPC server error", zap.Error(err))
	}
}
