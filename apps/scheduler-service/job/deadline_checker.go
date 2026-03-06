package job

import (
	"context"

	"go.uber.org/zap"
)

// DeadlineChecker publishes deadline.reminder for tasks due within 24h.
// Demo: publishes a synthetic event; real impl would query task-service via gRPC.
func DeadlineChecker(pub *Publisher, logger *zap.Logger) func(ctx context.Context) {
	return func(ctx context.Context) {
		payload := map[string]interface{}{
			"source":  "scheduler",
			"message": "deadline reminder check triggered",
		}
		if err := pub.Publish(ctx, "deadline.reminder", payload); err != nil {
			logger.Error("publishing deadline.reminder", zap.Error(err))
			return
		}
		logger.Info("deadline.reminder published")
	}
}
