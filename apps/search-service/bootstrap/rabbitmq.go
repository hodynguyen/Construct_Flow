package bootstrap

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	ExchangeName      = "constructflow.events"
	QueueSearchIndex  = "search_index_queue"
	DLQSearchIndex    = "search_index_queue.dlq"
	MaxRetries        = 3
)

// indexRoutingKeys returns the events that search-service should index.
func IndexRoutingKeys() []string {
	return []string{
		"task.assigned",
		"task.status_changed",
		"file.uploaded",
		"report.completed",
	}
}

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
	if err := ch.ExchangeDeclare(ExchangeName, "direct", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declaring exchange: %w", err)
	}

	dlxName := ExchangeName + ".dlx"
	if err := ch.ExchangeDeclare(dlxName, "direct", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declaring DLX: %w", err)
	}

	if _, err := ch.QueueDeclare(DLQSearchIndex, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declaring DLQ: %w", err)
	}
	for _, rk := range IndexRoutingKeys() {
		if err := ch.QueueBind(DLQSearchIndex, rk, dlxName, false, nil); err != nil {
			return fmt.Errorf("binding DLQ for %s: %w", rk, err)
		}
	}

	args := amqp.Table{"x-dead-letter-exchange": dlxName}
	if _, err := ch.QueueDeclare(QueueSearchIndex, true, false, false, false, args); err != nil {
		return fmt.Errorf("declaring search queue: %w", err)
	}
	for _, rk := range IndexRoutingKeys() {
		if err := ch.QueueBind(QueueSearchIndex, rk, ExchangeName, false, nil); err != nil {
			return fmt.Errorf("binding search queue for %s: %w", rk, err)
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
