package domain

import (
	"context"

	"github.com/hodynguyen/construct-flow/apps/notification-service/entity/model"
)

type ListNotificationFilter struct {
	UnreadOnly bool
}

type NotificationRepository interface {
	Create(ctx context.Context, n *model.Notification) error
	FindByID(ctx context.Context, notificationID, userID string) (*model.Notification, error)
	ListByUser(ctx context.Context, userID, companyID string, filter ListNotificationFilter, page, pageSize int) ([]model.Notification, int64, error)
	MarkRead(ctx context.Context, notificationID, userID string) error
	MarkAllRead(ctx context.Context, userID, companyID string) error
	CountUnread(ctx context.Context, userID string) (int64, error)
}
