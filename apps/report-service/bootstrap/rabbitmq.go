package bootstrap

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	ExchangeName          = "constructflow.events"
	QueueReportRequested  = "report_requested_queue"
	DLQReportRequested    = "report_requested_queue.dlq"
	RoutingKeyRequested   = "report.requested"
	RoutingKeyCompleted   = "report.completed"
	MaxRetries            = 3
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
	if err := ch.ExchangeDeclare(ExchangeName, "direct", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declaring exchange: %w", err)
	}

	dlxName := ExchangeName + ".dlx"
	if err := ch.ExchangeDeclare(dlxName, "direct", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declaring DLX: %w", err)
	}

	// DLQ
	if _, err := ch.QueueDeclare(DLQReportRequested, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declaring DLQ: %w", err)
	}
	if err := ch.QueueBind(DLQReportRequested, RoutingKeyRequested, dlxName, false, nil); err != nil {
		return fmt.Errorf("binding DLQ: %w", err)
	}

	// Main queue
	args := amqp.Table{
		"x-dead-letter-exchange":    dlxName,
		"x-dead-letter-routing-key": RoutingKeyRequested,
	}
	if _, err := ch.QueueDeclare(QueueReportRequested, true, false, false, false, args); err != nil {
		return fmt.Errorf("declaring queue: %w", err)
	}
	if err := ch.QueueBind(QueueReportRequested, RoutingKeyRequested, ExchangeName, false, nil); err != nil {
		return fmt.Errorf("binding queue: %w", err)
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
