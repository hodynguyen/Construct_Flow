package bootstrap

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	ExchangeName            = "constructflow.events"
	QueueTaskAssigned       = "task_assigned_queue"
	QueueTaskStatusChanged  = "task_status_changed_queue"
	RoutingKeyTaskAssigned  = "task.assigned"
	RoutingKeyStatusChanged = "task.status_changed"
)

type RabbitMQ struct {
	Conn    *amqp.Connection
	Channel *amqp.Channel
}

func NewRabbitMQ(cfg *Config) (*RabbitMQ, error) {
	conn, err := amqp.Dial(cfg.RabbitMQURL)
	if err != nil {
		return nil, fmt.Errorf("connecting to rabbitmq: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("opening rabbitmq channel: %w", err)
	}

	// Declare durable direct exchange
	if err := ch.ExchangeDeclare(
		ExchangeName,
		"direct",
		true,  // durable — survives broker restart
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,
	); err != nil {
		return nil, fmt.Errorf("declaring exchange: %w", err)
	}

	return &RabbitMQ{Conn: conn, Channel: ch}, nil
}

func (r *RabbitMQ) Close() {
	if r.Channel != nil {
		r.Channel.Close()
	}
	if r.Conn != nil {
		r.Conn.Close()
	}
}
