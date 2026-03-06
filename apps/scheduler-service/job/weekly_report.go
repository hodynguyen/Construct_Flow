package job

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// WeeklyReport publishes report.requested every Monday at 8am for all active companies.
// Demo: publishes for a synthetic company; real impl queries user-service for active tenants.
func WeeklyReport(pub *Publisher, logger *zap.Logger) func(ctx context.Context) {
	return func(ctx context.Context) {
		year, week := time.Now().ISOWeek()
		payload := map[string]interface{}{
			"source":   "scheduler",
			"job_type": "weekly_progress",
			"week":     fmt.Sprintf("%d-W%02d", year, week),
		}
		if err := pub.Publish(ctx, "report.requested", payload); err != nil {
			logger.Error("publishing weekly report.requested", zap.Error(err))
			return
		}
		logger.Info("weekly report.requested published", zap.String("week", payload["week"].(string)))
	}
}
