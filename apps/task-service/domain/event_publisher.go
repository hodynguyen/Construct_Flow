package domain

import "context"

// EventPublisher publishes domain events to the message broker (RabbitMQ).
// The domain layer depends only on this interface — never on the concrete implementation.
type EventPublisher interface {
	Publish(ctx context.Context, eventType string, payload interface{}) error
}
