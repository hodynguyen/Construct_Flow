package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.elastic.co/apm/v2"
	"go.uber.org/zap"

	"github.com/hodynguyen/construct-flow/apps/report-service/bootstrap"
	"github.com/hodynguyen/construct-flow/apps/report-service/domain"
	"github.com/hodynguyen/construct-flow/apps/report-service/service/storage"
)

type eventEnvelope struct {
	EventID   string          `json:"event_id"`
	EventType string          `json:"event_type"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

type reportRequestedPayload struct {
	JobID     string `json:"job_id"`
	CompanyID string `json:"company_id"`
	JobType   string `json:"job_type"`
	Params    string `json:"params"`
}

// ReportRequestedConsumer generates reports and uploads them to MinIO.
type ReportRequestedConsumer struct {
	channel   *amqp.Channel
	repo      domain.ReportRepository
	storage   *storage.MinIOClient
	publisher domain.EventPublisher
	logger    *zap.Logger
}

func NewReportRequestedConsumer(
	ch *amqp.Channel,
	repo domain.ReportRepository,
	storage *storage.MinIOClient,
	publisher domain.EventPublisher,
	logger *zap.Logger,
) *ReportRequestedConsumer {
	return &ReportRequestedConsumer{
		channel:   ch,
		repo:      repo,
		storage:   storage,
		publisher: publisher,
		logger:    logger,
	}
}

func (c *ReportRequestedConsumer) Start(ctx context.Context) {
	msgs, err := c.channel.Consume(
		bootstrap.QueueReportRequested,
		"report-service.report-requested",
		false, false, false, false, nil,
	)
	if err != nil {
		c.logger.Fatal("failed to start consumer",
			zap.String("queue", bootstrap.QueueReportRequested), zap.Error(err))
	}

	c.logger.Info("report_requested consumer started")

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-msgs:
			if !ok {
				c.logger.Warn("report_requested channel closed")
				return
			}
			c.processMessage(ctx, msg)
		}
	}
}

func (c *ReportRequestedConsumer) processMessage(ctx context.Context, msg amqp.Delivery) {
	tx := apm.DefaultTracer().StartTransaction("report.requested consume", "messaging")
	defer tx.End()
	ctx = apm.ContextWithTransaction(ctx, tx)

	var env eventEnvelope
	if err := json.Unmarshal(msg.Body, &env); err != nil {
		c.logger.Error("malformed message", zap.Error(err))
		_ = msg.Nack(false, false)
		return
	}

	var payload reportRequestedPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		c.logger.Error("malformed payload", zap.Error(err))
		_ = msg.Nack(false, false)
		return
	}

	retryCount := retryCountFromHeaders(msg.Headers)

	if err := c.generate(ctx, payload); err != nil {
		c.logger.Error("report generation failed",
			zap.String("job_id", payload.JobID),
			zap.Int("retry", retryCount),
			zap.Error(err),
		)
		if retryCount >= bootstrap.MaxRetries {
			errMsg := err.Error()
			_ = c.repo.UpdateStatus(ctx, payload.JobID, "failed", "", "", errMsg)
			_ = msg.Nack(false, false)
		} else {
			time.Sleep(time.Duration(1<<retryCount) * time.Second)
			_ = msg.Nack(false, true)
		}
		return
	}

	_ = msg.Ack(false)
	c.logger.Info("report generated", zap.String("job_id", payload.JobID))
}

func (c *ReportRequestedConsumer) generate(ctx context.Context, p reportRequestedPayload) error {
	// Mark as processing
	if err := c.repo.UpdateStatus(ctx, p.JobID, "processing", "", "", ""); err != nil {
		return fmt.Errorf("marking processing: %w", err)
	}

	// Generate report content (demo: JSON summary)
	report := map[string]interface{}{
		"job_id":       p.JobID,
		"company_id":   p.CompanyID,
		"type":         p.JobType,
		"params":       p.Params,
		"generated_at": time.Now().UTC(),
		"summary":      fmt.Sprintf("Report of type '%s' generated successfully (demo data).", p.JobType),
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling report: %w", err)
	}

	// Upload to MinIO
	s3Key := fmt.Sprintf("%s/reports/%s.json", p.CompanyID, p.JobID)
	if err := c.storage.UploadJSON(ctx, s3Key, data); err != nil {
		return fmt.Errorf("uploading report: %w", err)
	}

	// Presigned download URL (15 min)
	downloadURL, err := c.storage.PresignGetURL(ctx, s3Key, 15*time.Minute)
	if err != nil {
		return fmt.Errorf("presigning download url: %w", err)
	}

	// Mark as ready
	if err := c.repo.UpdateStatus(ctx, p.JobID, "ready", s3Key, downloadURL, ""); err != nil {
		return fmt.Errorf("marking ready: %w", err)
	}

	// Fetch job to get requestedBy for the completed event
	job, err := c.repo.FindByID(ctx, p.CompanyID, p.JobID)
	if err != nil {
		return nil // status already updated; non-critical
	}
	_ = c.publisher.PublishReportCompleted(ctx, p.JobID, p.CompanyID, job.RequestedBy, downloadURL)

	return nil
}

func retryCountFromHeaders(headers amqp.Table) int {
	if headers == nil {
		return 0
	}
	deaths, ok := headers["x-death"]
	if !ok {
		return 0
	}
	if list, ok := deaths.([]interface{}); ok {
		for _, item := range list {
			if table, ok := item.(amqp.Table); ok {
				if count, ok := table["count"].(int64); ok {
					return int(count)
				}
			}
		}
	}
	return 0
}
