package main

import (
	"log"

	"github.com/hodynguyen/construct-flow/apps/user-service/bootstrap"
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

	grpcServer := bootstrap.NewGRPCServer()

	log.Printf("user-service gRPC listening on :%d", cfg.GRPCPort)
	if err := bootstrap.StartGRPCServer(grpcServer, cfg.GRPCPort); err != nil {
		log.Fatalf("starting gRPC server: %v", err)
	}
}
