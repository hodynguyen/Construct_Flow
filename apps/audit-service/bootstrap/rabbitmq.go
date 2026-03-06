package bootstrap

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	ExchangeName  = "constructflow.events"
	QueueAudit    = "audit_queue"
	DLQAudit      = "audit_queue.dlq"
	MaxRetries    = 3
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
		return nil, fmt.Errorf("opening channel: %w", err)
	}

	if err := declareTopology(ch); err != nil {
		ch.Close()
		conn.Close()
		return nil, err
	}

	return &RabbitMQ{Conn: conn, Channel: ch}, nil
}

func declareTopology(ch *amqp.Channel) error {
	// Main exchange (already declared by other services — idempotent)
	if err := ch.ExchangeDeclare(ExchangeName, "direct", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declaring exchange: %w", err)
	}

	dlxName := ExchangeName + ".dlx"
	if err := ch.ExchangeDeclare(dlxName, "direct", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declaring DLX: %w", err)
	}

	// DLQ
	if _, err := ch.QueueDeclare(DLQAudit, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declaring DLQ: %w", err)
	}
	// Bind DLQ to ALL routing keys audit service cares about
	for _, rk := range auditRoutingKeys() {
		if err := ch.QueueBind(DLQAudit, rk, dlxName, false, nil); err != nil {
			return fmt.Errorf("binding DLQ for %s: %w", rk, err)
		}
	}

	// Main audit queue with DLX
	args := amqp.Table{
		"x-dead-letter-exchange": dlxName,
	}
	if _, err := ch.QueueDeclare(QueueAudit, true, false, false, false, args); err != nil {
		return fmt.Errorf("declaring audit queue: %w", err)
	}
	// Audit subscribes to ALL domain events (fan-out pattern via multiple bindings)
	for _, rk := range auditRoutingKeys() {
		if err := ch.QueueBind(QueueAudit, rk, ExchangeName, false, nil); err != nil {
			return fmt.Errorf("binding audit queue for %s: %w", rk, err)
		}
	}

	return nil
}

// auditRoutingKeys returns all event routing keys that audit-service must capture.
func auditRoutingKeys() []string {
	return []string{
		"task.assigned",
		"task.status_changed",
		"file.uploaded",
		"file.deleted",
		"report.requested",
		"report.completed",
		"user.login",
	}
}

func (r *RabbitMQ) Close() {
	if r.Channel != nil {
		r.Channel.Close()
	}
	if r.Conn != nil {
		r.Conn.Close()
	}
}
