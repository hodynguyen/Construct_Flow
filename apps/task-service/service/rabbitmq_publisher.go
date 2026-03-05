package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/hodynguyen/construct-flow/apps/task-service/domain"
)

const exchangeName = "constructflow.events"

type rabbitmqPublisher struct {
	channel *amqp.Channel
}

// NewRabbitMQPublisher returns an EventPublisher backed by RabbitMQ.
// Takes the amqp channel directly so this layer doesn't import bootstrap.
func NewRabbitMQPublisher(ch *amqp.Channel) domain.EventPublisher {
	return &rabbitmqPublisher{channel: ch}
}

type envelope struct {
	EventID   string      `json:"event_id"`
	EventType string      `json:"event_type"`
	Timestamp time.Time   `json:"timestamp"`
	Payload   interface{} `json:"payload"`
}

func (p *rabbitmqPublisher) Publish(ctx context.Context, eventType string, payload interface{}) error {
	msg := envelope{
		EventID:   uuid.New().String(),
		EventType: eventType,
		Timestamp: time.Now().UTC(),
		Payload:   payload,
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshaling event %s: %w", eventType, err)
	}

	return p.channel.PublishWithContext(ctx,
		exchangeName, // exchange
		eventType,    // routing key  e.g. "task.assigned"
		false,        // mandatory
		false,        // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent, // survive broker restart
			Body:         body,
		},
	)
}
