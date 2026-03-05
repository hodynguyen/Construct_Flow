package create_notification

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/hodynguyen/construct-flow/apps/notification-service/common"
	"github.com/hodynguyen/construct-flow/apps/notification-service/domain"
	"github.com/hodynguyen/construct-flow/apps/notification-service/entity/model"
)

const idempotencyTTL = 24 * time.Hour

// CreateNotificationInput carries data extracted from a RabbitMQ event envelope.
type CreateNotificationInput struct {
	EventID   string // idempotency key from the event envelope
	UserID    string
	CompanyID string
	Type      string // task_assigned | status_changed
	Title     string
	Message   string
	Metadata  string // JSON string
}

// UseCase creates a notification, skipping duplicates via Redis idempotency check.
type UseCase struct {
	repo  domain.NotificationRepository
	redis *redis.Client
}

func New(repo domain.NotificationRepository, redis *redis.Client) *UseCase {
	return &UseCase{repo: repo, redis: redis}
}

func (uc *UseCase) Execute(ctx context.Context, input CreateNotificationInput) error {
	if input.EventID == "" || input.UserID == "" || input.CompanyID == "" {
		return common.ErrInvalidPayload
	}

	// Idempotency check — SET NX with 24h TTL
	// If the key already exists, this event was already processed → skip.
	idempotencyKey := fmt.Sprintf("event:%s", input.EventID)
	set, err := uc.redis.SetNX(ctx, idempotencyKey, 1, idempotencyTTL).Result()
	if err != nil {
		// Redis unavailable — proceed anyway (risk of duplicate, acceptable tradeoff)
		// In production: use DB-level unique constraint on event_id instead.
	} else if !set {
		return common.ErrDuplicateEvent
	}

	n := &model.Notification{
		UserID:    input.UserID,
		CompanyID: input.CompanyID,
		Type:      input.Type,
		Title:     input.Title,
		Message:   input.Message,
		Metadata:  input.Metadata,
	}
	return uc.repo.Create(ctx, n)
}
