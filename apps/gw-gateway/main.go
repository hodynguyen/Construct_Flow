// @title           ConstructFlow API
// @version         1.0
// @description     Multi-tenant construction task management system.
// @host            localhost:8080
// @BasePath        /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description "Bearer <JWT token>"
package main

import (
	"fmt"
	"log"

	_ "github.com/hodynguyen/construct-flow/apps/gw-gateway/docs"
	gwhttp "github.com/hodynguyen/construct-flow/apps/gw-gateway/api/http"
	"github.com/hodynguyen/construct-flow/apps/gw-gateway/api/http/middleware"
	"github.com/hodynguyen/construct-flow/apps/gw-gateway/bootstrap"
	notifv1 "github.com/hodynguyen/construct-flow/gen/go/proto/notification_service/v1"
	taskv1 "github.com/hodynguyen/construct-flow/gen/go/proto/task_service/v1"
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

	enforcer, err := middleware.NewCasbinEnforcer(cfg.CasbinModelPath, cfg.CasbinPolicyPath)
	if err != nil {
		log.Fatalf("loading casbin enforcer: %v", err)
	}

	router, err := gwhttp.NewRouter(gwhttp.RouterConfig{
		UserClient:       userv1.NewUserServiceClient(grpcClients.UserServiceConn),
		TaskClient:       taskv1.NewTaskServiceClient(grpcClients.TaskServiceConn),
		NotifClient:      notifv1.NewNotificationServiceClient(grpcClients.NotificationServiceConn),
		RedisClient:      redisClient,
		Enforcer:         enforcer,
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
