package consumer

import (
	"context"
	"encoding/json"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.elastic.co/apm/v2"
	"go.uber.org/zap"

	"github.com/hodynguyen/construct-flow/apps/audit-service/bootstrap"
	"github.com/hodynguyen/construct-flow/apps/audit-service/domain"
	"github.com/hodynguyen/construct-flow/apps/audit-service/entity/model"
)

type eventEnvelope struct {
	EventID   string          `json:"event_id"`
	EventType string          `json:"event_type"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

type genericPayload struct {
	CompanyID  string `json:"company_id"`
	UserID     string `json:"user_id"`
	ResourceID string `json:"task_id,omitempty"`    // from task events
	FileID     string `json:"file_id,omitempty"`    // from file events
	JobID      string `json:"job_id,omitempty"`     // from report events
}

// EventConsumer consumes all domain events and appends to the audit log.
type EventConsumer struct {
	channel *amqp.Channel
	repo    domain.AuditRepository
	logger  *zap.Logger
}

func NewEventConsumer(ch *amqp.Channel, repo domain.AuditRepository, logger *zap.Logger) *EventConsumer {
	return &EventConsumer{channel: ch, repo: repo, logger: logger}
}

func (c *EventConsumer) Start(ctx context.Context) {
	msgs, err := c.channel.Consume(
		bootstrap.QueueAudit,
		"audit-service.all-events",
		false, false, false, false, nil,
	)
	if err != nil {
		c.logger.Fatal("failed to start audit consumer", zap.Error(err))
	}

	c.logger.Info("audit event consumer started")

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-msgs:
			if !ok {
				c.logger.Warn("audit queue channel closed")
				return
			}
			c.processMessage(ctx, msg)
		}
	}
}

func (c *EventConsumer) processMessage(ctx context.Context, msg amqp.Delivery) {
	tx := apm.DefaultTracer().StartTransaction("audit.event consume", "messaging")
	defer tx.End()
	ctx = apm.ContextWithTransaction(ctx, tx)

	var env eventEnvelope
	if err := json.Unmarshal(msg.Body, &env); err != nil {
		c.logger.Error("malformed audit event", zap.Error(err))
		_ = msg.Nack(false, false)
		return
	}

	var p genericPayload
	_ = json.Unmarshal(env.Payload, &p)

	resourceID := p.ResourceID
	if resourceID == "" {
		resourceID = p.FileID
	}
	if resourceID == "" {
		resourceID = p.JobID
	}

	resource := resourceFromEventType(env.EventType)

	log := &model.AuditLog{
		CompanyID:  p.CompanyID,
		UserID:     p.UserID,
		Action:     env.EventType,
		Resource:   resource,
		ResourceID: resourceID,
		AfterState: string(env.Payload),
		OccurredAt: env.Timestamp,
	}

	retryCount := retryCountFromHeaders(msg.Headers)

	if err := c.repo.Append(ctx, log); err != nil {
		c.logger.Error("failed to append audit log",
			zap.String("event_type", env.EventType),
			zap.Int("retry", retryCount),
			zap.Error(err),
		)
		if retryCount >= bootstrap.MaxRetries {
			_ = msg.Nack(false, false) // to DLQ
		} else {
			time.Sleep(time.Duration(1<<retryCount) * time.Second)
			_ = msg.Nack(false, true)
		}
		return
	}

	_ = msg.Ack(false)
}

func resourceFromEventType(eventType string) string {
	switch {
	case len(eventType) >= 4 && eventType[:4] == "task":
		return "task"
	case len(eventType) >= 4 && eventType[:4] == "file":
		return "file"
	case len(eventType) >= 6 && eventType[:6] == "report":
		return "report"
	case len(eventType) >= 4 && eventType[:4] == "user":
		return "user"
	default:
		return "unknown"
	}
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
