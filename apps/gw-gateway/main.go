package main

import (
	"fmt"
	"log"

	"github.com/hodynguyen/construct-flow/apps/gw-gateway/bootstrap"
	gwhttp "github.com/hodynguyen/construct-flow/apps/gw-gateway/api/http"
	userv1 "github.com/hodynguyen/construct-flow/gen/go/proto/user_service/v1"
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

	grpcClients, err := bootstrap.NewGRPCClients(cfg)
	if err != nil {
		log.Fatalf("connecting to backend services: %v", err)
	}
	defer grpcClients.Close()

	router, err := gwhttp.NewRouter(gwhttp.RouterConfig{
		UserClient:       userv1.NewUserServiceClient(grpcClients.UserServiceConn),
		RedisClient:      redisClient,
		RateLimitRPM:     cfg.RateLimitRequestsPerMinute,
		JWTPublicKeyPath: cfg.JWTPublicKeyPath,
	})
	if err != nil {
		log.Fatalf("building router: %v", err)
	}

	addr := fmt.Sprintf(":%d", cfg.HTTPPort)
	log.Printf("gw-gateway HTTP listening on %s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatalf("starting HTTP server: %v", err)
	}
}
