package job

import (
	"context"

	"go.uber.org/zap"
)

// FileLifecycle triggers cold storage migration for eligible files.
// Demo: publishes a synthetic event; real impl calls file-service.MigrateStorage via gRPC.
func FileLifecycle(pub *Publisher, logger *zap.Logger) func(ctx context.Context) {
	return func(ctx context.Context) {
		payload := map[string]interface{}{
			"source":  "scheduler",
			"message": "file lifecycle migration triggered",
		}
		if err := pub.Publish(ctx, "file.lifecycle", payload); err != nil {
			logger.Error("publishing file.lifecycle", zap.Error(err))
			return
		}
		logger.Info("file.lifecycle published")
	}
}
