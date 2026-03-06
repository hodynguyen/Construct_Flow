package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/hodynguyen/construct-flow/apps/report-service/bootstrap"
)

type reportEventPublisher struct {
	channel *amqp.Channel
}

func NewReportEventPublisher(ch *amqp.Channel) *reportEventPublisher {
	return &reportEventPublisher{channel: ch}
}

func (p *reportEventPublisher) PublishReportRequested(ctx context.Context, jobID, companyID, jobType, params string) error {
	payload := map[string]string{
		"job_id":     jobID,
		"company_id": companyID,
		"job_type":   jobType,
		"params":     params,
	}
	return p.publish(ctx, bootstrap.RoutingKeyRequested, jobID, "report.requested", payload)
}

func (p *reportEventPublisher) PublishReportCompleted(ctx context.Context, jobID, companyID, requestedBy, downloadURL string) error {
	payload := map[string]string{
		"job_id":       jobID,
		"company_id":   companyID,
		"requested_by": requestedBy,
		"download_url": downloadURL,
	}
	return p.publish(ctx, bootstrap.RoutingKeyCompleted, jobID, "report.completed", payload)
}

type eventEnvelope struct {
	EventID   string      `json:"event_id"`
	EventType string      `json:"event_type"`
	Timestamp time.Time   `json:"timestamp"`
	Payload   interface{} `json:"payload"`
}

func (p *reportEventPublisher) publish(ctx context.Context, routingKey, eventID, eventType string, payload interface{}) error {
	env := eventEnvelope{
		EventID:   fmt.Sprintf("%s-%d", eventID, time.Now().UnixNano()),
		EventType: eventType,
		Timestamp: time.Now().UTC(),
		Payload:   payload,
	}
	body, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshaling event: %w", err)
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
