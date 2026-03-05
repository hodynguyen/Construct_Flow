package main

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/hodynguyen/construct-flow/apps/gw-gateway/bootstrap"
)

func main() {
	cfg, err := bootstrap.LoadConfig()
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	redisClient, err := bootstrap.NewRedisClient(cfg)
	if err != nil {
		log.Fatalf("connecting to redis: %v", err)
	}
	_ = redisClient

	grpcClients, err := bootstrap.NewGRPCClients(cfg)
	if err != nil {
		log.Fatalf("connecting to backend services: %v", err)
	}
	defer grpcClients.Close()

	router := gin.Default()

	// Routes will be registered here in Day 3
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	addr := fmt.Sprintf(":%d", cfg.HTTPPort)
	log.Printf("gw-gateway HTTP listening on %s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatalf("starting HTTP server: %v", err)
	}
}
