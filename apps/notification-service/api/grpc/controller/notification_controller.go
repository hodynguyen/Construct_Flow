package controller

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/hodynguyen/construct-flow/apps/notification-service/common"
	"github.com/hodynguyen/construct-flow/apps/notification-service/domain"
	"github.com/hodynguyen/construct-flow/apps/notification-service/entity/model"
	"github.com/hodynguyen/construct-flow/apps/notification-service/use-case/mark_read"
	notifv1 "github.com/hodynguyen/construct-flow/gen/go/proto/notification_service/v1"
)

// NotificationController implements the gRPC NotificationServiceServer.
type NotificationController struct {
	notifv1.UnimplementedNotificationServiceServer
	repo       domain.NotificationRepository
	markReadUC *mark_read.UseCase
}

func NewNotificationController(repo domain.NotificationRepository, markReadUC *mark_read.UseCase) *NotificationController {
	return &NotificationController{repo: repo, markReadUC: markReadUC}
}

func (c *NotificationController) GetNotifications(ctx context.Context, req *notifv1.GetNotificationsRequest) (*notifv1.NotificationsResponse, error) {
	page := int(req.Page)
	pageSize := int(req.PageSize)

	notifications, total, err := c.repo.ListByUser(ctx, req.UserId, req.CompanyId,
		domain.ListNotificationFilter{UnreadOnly: req.UnreadOnly}, page, pageSize)
	if err != nil {
		return nil, status.Error(codes.Internal, "internal error")
	}

	var protoNotifs []*notifv1.Notification
	for i := range notifications {
		protoNotifs = append(protoNotifs, toProtoNotification(&notifications[i]))
	}

	return &notifv1.NotificationsResponse{
		Notifications: protoNotifs,
		Total:         int32(total),
		Page:          req.Page,
		PageSize:      req.PageSize,
	}, nil
}

func (c *NotificationController) GetUnreadCount(ctx context.Context, req *notifv1.GetUnreadCountRequest) (*notifv1.UnreadCountResponse, error) {
	count, err := c.repo.CountUnread(ctx, req.UserId)
	if err != nil {
		return nil, status.Error(codes.Internal, "internal error")
	}
	return &notifv1.UnreadCountResponse{Count: int32(count)}, nil
}

func (c *NotificationController) MarkAsRead(ctx context.Context, req *notifv1.MarkAsReadRequest) (*emptypb.Empty, error) {
	if err := c.markReadUC.Execute(ctx, req.NotificationId, req.UserId); err != nil {
		if errors.Is(err, common.ErrNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, "internal error")
	}
	return &emptypb.Empty{}, nil
}

func (c *NotificationController) MarkAllAsRead(ctx context.Context, req *notifv1.MarkAllAsReadRequest) (*emptypb.Empty, error) {
	if err := c.repo.MarkAllRead(ctx, req.UserId, req.CompanyId); err != nil {
		return nil, status.Error(codes.Internal, "internal error")
	}
	return &emptypb.Empty{}, nil
}

func toProtoNotification(n *model.Notification) *notifv1.Notification {
	return &notifv1.Notification{
		Id:        n.ID,
		UserId:    n.UserID,
		CompanyId: n.CompanyID,
		Type:      n.Type,
		Title:     n.Title,
		Message:   n.Message,
		IsRead:    n.IsRead,
		Metadata:  n.Metadata,
		CreatedAt: timestamppb.New(n.CreatedAt),
	}
}
