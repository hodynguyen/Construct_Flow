package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.elastic.co/apm/v2"
	"go.uber.org/zap"

	"github.com/hodynguyen/construct-flow/apps/notification-service/bootstrap"
	"github.com/hodynguyen/construct-flow/apps/notification-service/common"
	"github.com/hodynguyen/construct-flow/apps/notification-service/use-case/create_notification"
)

// eventEnvelope matches the envelope format published by task-service.
type eventEnvelope struct {
	EventID   string          `json:"event_id"`
	EventType string          `json:"event_type"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

type taskAssignedPayload struct {
	TaskID     string `json:"task_id"`
	AssignedTo string `json:"assigned_to"`
	CompanyID  string `json:"company_id"`
}

// TaskAssignedConsumer consumes task.assigned events and creates notifications.
type TaskAssignedConsumer struct {
	channel        *amqp.Channel
	createNotifUC  *create_notification.UseCase
	logger         *zap.Logger
}

func NewTaskAssignedConsumer(ch *amqp.Channel, uc *create_notification.UseCase, logger *zap.Logger) *TaskAssignedConsumer {
	return &TaskAssignedConsumer{channel: ch, createNotifUC: uc, logger: logger}
}

// Start begins consuming from task_assigned_queue. Blocks until ctx is cancelled.
func (c *TaskAssignedConsumer) Start(ctx context.Context) {
	msgs, err := c.channel.Consume(
		bootstrap.QueueTaskAssigned,
		"notification-service.task-assigned", // consumer tag
		false, false, false, false, nil,
	)
	if err != nil {
		c.logger.Fatal("failed to start consumer", zap.String("queue", bootstrap.QueueTaskAssigned), zap.Error(err))
	}

	c.logger.Info("task_assigned consumer started", zap.String("queue", bootstrap.QueueTaskAssigned))

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("task_assigned consumer stopping")
			return
		case msg, ok := <-msgs:
			if !ok {
				c.logger.Warn("task_assigned_queue channel closed")
				return
			}
			c.processMessage(ctx, msg)
		}
	}
}

func (c *TaskAssignedConsumer) processMessage(ctx context.Context, msg amqp.Delivery) {
	tx := apm.DefaultTracer().StartTransaction("task.assigned consume", "messaging")
	defer tx.End()
	ctx = apm.ContextWithTransaction(ctx, tx)

	var env eventEnvelope
	if err := json.Unmarshal(msg.Body, &env); err != nil {
		c.logger.Error("malformed message body — sending to DLQ", zap.Error(err))
		msg.Nack(false, false) // requeue=false → routes to DLQ
		return
	}

	var payload taskAssignedPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		c.logger.Error("malformed task.assigned payload", zap.String("event_id", env.EventID), zap.Error(err))
		msg.Nack(false, false)
		return
	}

	retryCount := retryCountFromHeaders(msg.Headers)

	err := c.createNotifUC.Execute(ctx, create_notification.CreateNotificationInput{
		EventID:   env.EventID,
		UserID:    payload.AssignedTo,
		CompanyID: payload.CompanyID,
		Type:      "task_assigned",
		Title:     "New task assigned to you",
		Message:   fmt.Sprintf("Task %s has been assigned to you", payload.TaskID),
		Metadata:  string(env.Payload),
	})

	if err == common.ErrDuplicateEvent {
		c.logger.Info("duplicate event skipped", zap.String("event_id", env.EventID))
		msg.Ack(false)
		return
	}

	if err != nil {
		c.logger.Error("failed to process task.assigned",
			zap.String("event_id", env.EventID),
			zap.Int("retry", retryCount),
			zap.Error(err),
		)
		if retryCount >= bootstrap.MaxRetries {
			c.logger.Error("max retries exceeded — sending to DLQ", zap.String("event_id", env.EventID))
			msg.Nack(false, false) // DLQ
		} else {
			// Exponential backoff: 1s, 2s, 4s
			time.Sleep(time.Duration(1<<retryCount) * time.Second)
			msg.Nack(false, true) // requeue=true for retry
		}
		return
	}

	msg.Ack(false)
	c.logger.Info("task.assigned processed", zap.String("event_id", env.EventID))
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
