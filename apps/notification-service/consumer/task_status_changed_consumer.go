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

type taskStatusChangedPayload struct {
	TaskID    string `json:"task_id"`
	NewStatus string `json:"new_status"`
	CompanyID string `json:"company_id"`
	UserID    string `json:"user_id"` // task assignee to notify
}

// TaskStatusChangedConsumer consumes task.status_changed events.
type TaskStatusChangedConsumer struct {
	channel       *amqp.Channel
	createNotifUC *create_notification.UseCase
	logger        *zap.Logger
}

func NewTaskStatusChangedConsumer(ch *amqp.Channel, uc *create_notification.UseCase, logger *zap.Logger) *TaskStatusChangedConsumer {
	return &TaskStatusChangedConsumer{channel: ch, createNotifUC: uc, logger: logger}
}

// Start begins consuming from task_status_changed_queue. Blocks until ctx is cancelled.
func (c *TaskStatusChangedConsumer) Start(ctx context.Context) {
	msgs, err := c.channel.Consume(
		bootstrap.QueueTaskStatusChanged,
		"notification-service.task-status-changed",
		false, false, false, false, nil,
	)
	if err != nil {
		c.logger.Fatal("failed to start consumer", zap.String("queue", bootstrap.QueueTaskStatusChanged), zap.Error(err))
	}

	c.logger.Info("task_status_changed consumer started", zap.String("queue", bootstrap.QueueTaskStatusChanged))

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("task_status_changed consumer stopping")
			return
		case msg, ok := <-msgs:
			if !ok {
				c.logger.Warn("task_status_changed_queue channel closed")
				return
			}
			c.processMessage(ctx, msg)
		}
	}
}

func (c *TaskStatusChangedConsumer) processMessage(ctx context.Context, msg amqp.Delivery) {
	tx := apm.DefaultTracer().StartTransaction("task.status_changed consume", "messaging")
	defer tx.End()
	ctx = apm.ContextWithTransaction(ctx, tx)

	var env eventEnvelope
	if err := json.Unmarshal(msg.Body, &env); err != nil {
		c.logger.Error("malformed message body", zap.Error(err))
		_ = msg.Nack(false, false)
		return
	}

	var payload taskStatusChangedPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		c.logger.Error("malformed task.status_changed payload", zap.String("event_id", env.EventID), zap.Error(err))
		_ = msg.Nack(false, false)
		return
	}

	// Only create notification if there is an assignee to notify
	if payload.UserID == "" {
		_ = msg.Ack(false)
		return
	}

	retryCount := retryCountFromHeaders(msg.Headers)

	err := c.createNotifUC.Execute(ctx, create_notification.CreateNotificationInput{
		EventID:   env.EventID,
		UserID:    payload.UserID,
		CompanyID: payload.CompanyID,
		Type:      "status_changed",
		Title:     "Task status updated",
		Message:   fmt.Sprintf("Task %s status changed to %s", payload.TaskID, payload.NewStatus),
		Metadata:  string(env.Payload),
	})

	if err == common.ErrDuplicateEvent {
		c.logger.Info("duplicate event skipped", zap.String("event_id", env.EventID))
		_ = msg.Ack(false)
		return
	}

	if err != nil {
		c.logger.Error("failed to process task.status_changed",
			zap.String("event_id", env.EventID),
			zap.Int("retry", retryCount),
			zap.Error(err),
		)
		if retryCount >= bootstrap.MaxRetries {
			_ = msg.Nack(false, false)
		} else {
			time.Sleep(time.Duration(1<<retryCount) * time.Second)
			_ = msg.Nack(false, true)
		}
		return
	}

	_ = msg.Ack(false)
	c.logger.Info("task.status_changed processed", zap.String("event_id", env.EventID))
}
