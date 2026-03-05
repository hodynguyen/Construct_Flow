package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/hodynguyen/construct-flow/apps/gw-gateway/api/http/handler"
	"github.com/hodynguyen/construct-flow/apps/gw-gateway/api/http/middleware"
	userv1 "github.com/hodynguyen/construct-flow/gen/go/proto/user_service/v1"
)

// RouterConfig holds all dependencies needed to register routes.
type RouterConfig struct {
	UserClient        userv1.UserServiceClient
	RedisClient       *redis.Client
	RateLimitRPM      int
	JWTPublicKeyPath  string
}

// NewRouter builds and returns the configured Gin engine.
func NewRouter(cfg RouterConfig) (*gin.Engine, error) {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.Logger())

	// Rate limiting — applied globally
	r.Use(middleware.RateLimitMiddleware(cfg.RedisClient, cfg.RateLimitRPM))

	// Health check (no auth)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	authMiddleware, err := middleware.AuthMiddleware(cfg.JWTPublicKeyPath)
	if err != nil {
		return nil, err
	}

	authHandler := handler.NewAuthHandler(cfg.UserClient)

	v1 := r.Group("/api/v1")
	{
		// Public auth routes
		auth := v1.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
		}

		// Protected routes — require valid JWT
		protected := v1.Group("")
		protected.Use(authMiddleware)
		{
			// Project and task routes will be wired in Day 4
			_ = protected
		}
	}

	return r, nil
}
