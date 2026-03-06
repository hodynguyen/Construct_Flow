package job

import (
	"context"

	"go.uber.org/zap"
)

// OverdueChecker publishes deadline.overdue for tasks past their due date.
func OverdueChecker(pub *Publisher, logger *zap.Logger) func(ctx context.Context) {
	return func(ctx context.Context) {
		payload := map[string]interface{}{
			"source":  "scheduler",
			"message": "overdue check triggered",
		}
		if err := pub.Publish(ctx, "deadline.overdue", payload); err != nil {
			logger.Error("publishing deadline.overdue", zap.Error(err))
			return
		}
		logger.Info("deadline.overdue published")
	}
}
