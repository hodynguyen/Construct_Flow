package bootstrap

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	ExchangeName           = "constructflow.events"
	QueueTaskAssigned      = "task_assigned_queue"
	QueueTaskStatusChanged = "task_status_changed_queue"
	DLQTaskAssigned        = "task_assigned_queue.dlq"
	DLQTaskStatusChanged   = "task_status_changed_queue.dlq"
	RoutingKeyAssigned     = "task.assigned"
	RoutingKeyStatusChanged = "task.status_changed"
	MaxRetries             = 3
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

	if err := declareTopology(ch); err != nil {
		ch.Close()
		conn.Close()
		return nil, err
	}

	return &RabbitMQ{Conn: conn, Channel: ch}, nil
}

func declareTopology(ch *amqp.Channel) error {
	// Main exchange
	if err := ch.ExchangeDeclare(ExchangeName, "direct", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declaring exchange: %w", err)
	}

	// DLQ exchange
	dlxName := ExchangeName + ".dlx"
	if err := ch.ExchangeDeclare(dlxName, "direct", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declaring DLX exchange: %w", err)
	}

	queues := []struct {
		name       string
		dlq        string
		routingKey string
	}{
		{QueueTaskAssigned, DLQTaskAssigned, RoutingKeyAssigned},
		{QueueTaskStatusChanged, DLQTaskStatusChanged, RoutingKeyStatusChanged},
	}

	for _, q := range queues {
		// DLQ first
		if _, err := ch.QueueDeclare(q.dlq, true, false, false, false, nil); err != nil {
			return fmt.Errorf("declaring DLQ %s: %w", q.dlq, err)
		}
		if err := ch.QueueBind(q.dlq, q.routingKey, dlxName, false, nil); err != nil {
			return fmt.Errorf("binding DLQ %s: %w", q.dlq, err)
		}

		// Main queue with dead-letter routing
		args := amqp.Table{
			"x-dead-letter-exchange":    dlxName,
			"x-dead-letter-routing-key": q.routingKey,
		}
		if _, err := ch.QueueDeclare(q.name, true, false, false, false, args); err != nil {
			return fmt.Errorf("declaring queue %s: %w", q.name, err)
		}
		if err := ch.QueueBind(q.name, q.routingKey, ExchangeName, false, nil); err != nil {
			return fmt.Errorf("binding queue %s: %w", q.name, err)
		}
	}

	return nil
}

func (r *RabbitMQ) Close() {
	if r.Channel != nil {
		r.Channel.Close()
	}
	if r.Conn != nil {
		r.Conn.Close()
	}
}
