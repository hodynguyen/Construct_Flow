package consumer

import (
	"context"
	"encoding/json"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.elastic.co/apm/v2"
	"go.uber.org/zap"

	"github.com/hodynguyen/construct-flow/apps/search-service/bootstrap"
	"github.com/hodynguyen/construct-flow/apps/search-service/service/elastic"
)

type eventEnvelope struct {
	EventID   string          `json:"event_id"`
	EventType string          `json:"event_type"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

// IndexerConsumer consumes domain events and indexes them in Elasticsearch.
type IndexerConsumer struct {
	channel *amqp.Channel
	es      *elastic.Client
	logger  *zap.Logger
}

func NewIndexerConsumer(ch *amqp.Channel, es *elastic.Client, logger *zap.Logger) *IndexerConsumer {
	return &IndexerConsumer{channel: ch, es: es, logger: logger}
}

func (c *IndexerConsumer) Start(ctx context.Context) {
	msgs, err := c.channel.Consume(
		bootstrap.QueueSearchIndex,
		"search-service.indexer",
		false, false, false, false, nil,
	)
	if err != nil {
		c.logger.Fatal("failed to start indexer consumer", zap.Error(err))
	}

	c.logger.Info("search indexer consumer started")

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-msgs:
			if !ok {
				c.logger.Warn("search index queue channel closed")
				return
			}
			c.processMessage(ctx, msg)
		}
	}
}

func (c *IndexerConsumer) processMessage(ctx context.Context, msg amqp.Delivery) {
	tx := apm.DefaultTracer().StartTransaction("search.index consume", "messaging")
	defer tx.End()
	ctx = apm.ContextWithTransaction(ctx, tx)

	var env eventEnvelope
	if err := json.Unmarshal(msg.Body, &env); err != nil {
		c.logger.Error("malformed search event", zap.Error(err))
		_ = msg.Nack(false, false)
		return
	}

	retryCount := retryCountFromHeaders(msg.Headers)

	if err := c.index(ctx, env); err != nil {
		c.logger.Error("indexing failed",
			zap.String("event_type", env.EventType),
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
}

func (c *IndexerConsumer) index(ctx context.Context, env eventEnvelope) error {
	var payload map[string]interface{}
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		return err
	}
	payload["indexed_at"] = time.Now().UTC()
	payload["event_type"] = env.EventType

	// Route to the correct Elasticsearch index based on event type
	esIndex, docID := routeEvent(env.EventType, payload)
	if esIndex == "" {
		// Unknown event type — skip silently
		return nil
	}

	return c.es.IndexDocument(ctx, esIndex, docID, payload)
}

func routeEvent(eventType string, payload map[string]interface{}) (index string, docID string) {
	getString := func(key string) string {
		if v, ok := payload[key].(string); ok {
			return v
		}
		return ""
	}

	switch {
	case len(eventType) >= 4 && eventType[:4] == "task":
		return "tasks", getString("task_id")
	case len(eventType) >= 4 && eventType[:4] == "file":
		return "files", getString("file_id")
	case len(eventType) >= 6 && eventType[:6] == "report":
		return "reports", getString("job_id")
	default:
		return "", ""
	}
}

func retryCountFromHeaders(headers amqp.Table) int {
	if headers == nil {
		return 0
	}
	deaths, ok := headers["x-death"]
	if !ok {
		return 0
	}
	if list, ok := deaths.([]interface{}); ok {
		for _, item := range list {
			if table, ok := item.(amqp.Table); ok {
				if count, ok := table["count"].(int64); ok {
					return int(count)
				}
			}
		}
	}
	return 0
}
