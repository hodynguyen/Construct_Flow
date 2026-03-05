package mark_read

import (
	"context"

	"github.com/hodynguyen/construct-flow/apps/notification-service/domain"
)

// UseCase marks a single notification as read (ownership verified at repo level).
type UseCase struct {
	repo domain.NotificationRepository
}

func New(repo domain.NotificationRepository) *UseCase {
	return &UseCase{repo: repo}
}

func (uc *UseCase) Execute(ctx context.Context, notificationID, userID string) error {
	return uc.repo.MarkRead(ctx, notificationID, userID)
}
