package http

import (
	"net/http"

	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.elastic.co/apm/module/apmgin/v2"

	"github.com/hodynguyen/construct-flow/apps/gw-gateway/api/http/handler"
	"github.com/hodynguyen/construct-flow/apps/gw-gateway/api/http/middleware"
	notifv1 "github.com/hodynguyen/construct-flow/gen/go/proto/notification_service/v1"
	taskv1 "github.com/hodynguyen/construct-flow/gen/go/proto/task_service/v1"
	userv1 "github.com/hodynguyen/construct-flow/gen/go/proto/user_service/v1"
)

// RouterConfig holds all dependencies needed to register routes.
type RouterConfig struct {
	UserClient       userv1.UserServiceClient
	TaskClient       taskv1.TaskServiceClient
	NotifClient      notifv1.NotificationServiceClient
	RedisClient      *redis.Client
	Enforcer         *casbin.Enforcer
	RateLimitRPM     int
	JWTPublicKeyPath string
}

// NewRouter builds and returns the configured Gin engine.
func NewRouter(cfg RouterConfig) (*gin.Engine, error) {
	r := gin.New()
	r.Use(apmgin.Middleware(r))
	r.Use(gin.Recovery())
	r.Use(gin.Logger())
	r.Use(middleware.RateLimitMiddleware(cfg.RedisClient, cfg.RateLimitRPM))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	authMiddleware, err := middleware.AuthMiddleware(cfg.JWTPublicKeyPath)
	if err != nil {
		return nil, err
	}

	// Handlers
	authHandler := handler.NewAuthHandler(cfg.UserClient)
	projectHandler := handler.NewProjectHandler(cfg.TaskClient)
	taskHandler := handler.NewTaskHandler(cfg.TaskClient)
	notifHandler := handler.NewNotificationHandler(cfg.NotifClient)

	v1 := r.Group("/api/v1")

	// ── Public auth routes ─────────────────────────────────────────────────
	auth := v1.Group("/auth")
	{
		auth.POST("/register", authHandler.Register)
		auth.POST("/login", authHandler.Login)
	}

	// ── Protected routes — JWT required ────────────────────────────────────
	protected := v1.Group("")
	protected.Use(authMiddleware)
	{
		// Projects
		projects := protected.Group("/projects")
		{
			projects.GET("",    middleware.RBACMiddleware(cfg.Enforcer, "/projects", "read"),  projectHandler.ListProjects)
			projects.POST("",   middleware.RBACMiddleware(cfg.Enforcer, "/projects", "write"), projectHandler.CreateProject)
			projects.GET("/:id",    middleware.RBACMiddleware(cfg.Enforcer, "/projects", "read"),  projectHandler.GetProject)
			projects.PUT("/:id",    middleware.RBACMiddleware(cfg.Enforcer, "/projects", "write"), projectHandler.UpdateProject)
			projects.DELETE("/:id", middleware.RBACMiddleware(cfg.Enforcer, "/projects", "write"), projectHandler.DeleteProject)

			// Tasks nested under project
			projects.GET("/:project_id/tasks",  middleware.RBACMiddleware(cfg.Enforcer, "/tasks", "read"),  taskHandler.ListTasks)
			projects.POST("/:project_id/tasks", middleware.RBACMiddleware(cfg.Enforcer, "/tasks", "write"), taskHandler.CreateTask)
		}

		// Tasks (standalone access)
		tasks := protected.Group("/tasks")
		{
			tasks.GET("/:id",           middleware.RBACMiddleware(cfg.Enforcer, "/tasks", "read"),         taskHandler.GetTask)
			tasks.POST("/:id/assign",   middleware.RBACMiddleware(cfg.Enforcer, "/tasks/assign", "write"), taskHandler.AssignTask)
			tasks.PATCH("/:id/status",  middleware.RBACMiddleware(cfg.Enforcer, "/tasks/status", "write"), taskHandler.UpdateTaskStatus)
		}

		// Notifications
		notifs := protected.Group("/notifications")
		{
			notifs.GET("",                   middleware.RBACMiddleware(cfg.Enforcer, "/notifications", "read"),  notifHandler.GetNotifications)
			notifs.GET("/unread/count",       middleware.RBACMiddleware(cfg.Enforcer, "/notifications", "read"),  notifHandler.GetUnreadCount)
			notifs.PATCH("/:id/read",         middleware.RBACMiddleware(cfg.Enforcer, "/notifications", "write"), notifHandler.MarkAsRead)
		}
	}

	return r, nil
}
