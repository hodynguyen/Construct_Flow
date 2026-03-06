package job

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/hodynguyen/construct-flow/apps/scheduler-service/bootstrap"
)

// Publisher publishes scheduler-triggered events to RabbitMQ.
type Publisher struct {
	channel *amqp.Channel
}

func NewPublisher(ch *amqp.Channel) *Publisher {
	return &Publisher{channel: ch}
}

func (p *Publisher) Publish(ctx context.Context, routingKey string, payload interface{}) error {
	env := map[string]interface{}{
		"event_id":   fmt.Sprintf("sched-%d", time.Now().UnixNano()),
		"event_type": routingKey,
		"timestamp":  time.Now().UTC(),
		"payload":    payload,
	}
	body, err := json.Marshal(env)
	if err != nil {
		return err
	}
	return p.channel.PublishWithContext(ctx,
		bootstrap.ExchangeName,
		routingKey,
		false, false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
		},
	)
}
