package sql

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/hodynguyen/construct-flow/apps/notification-service/common"
	"github.com/hodynguyen/construct-flow/apps/notification-service/domain"
	"github.com/hodynguyen/construct-flow/apps/notification-service/entity/model"
)

type notificationRepository struct {
	db *gorm.DB
}

// NewNotificationRepository returns a GORM-backed NotificationRepository.
func NewNotificationRepository(db *gorm.DB) domain.NotificationRepository {
	return &notificationRepository{db: db}
}

func (r *notificationRepository) Create(ctx context.Context, n *model.Notification) error {
	return r.db.WithContext(ctx).Create(n).Error
}

func (r *notificationRepository) FindByID(ctx context.Context, notificationID, userID string) (*model.Notification, error) {
	var n model.Notification
	err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", notificationID, userID).
		First(&n).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, common.ErrNotFound
	}
	return &n, err
}

func (r *notificationRepository) ListByUser(ctx context.Context, userID, companyID string, filter domain.ListNotificationFilter, page, pageSize int) ([]model.Notification, int64, error) {
	q := r.db.WithContext(ctx).Where("user_id = ? AND company_id = ?", userID, companyID)
	if filter.UnreadOnly {
		q = q.Where("is_read = false")
	}

	var total int64
	if err := q.Model(&model.Notification{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var notifications []model.Notification
	err := q.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&notifications).Error
	return notifications, total, err
}

func (r *notificationRepository) MarkRead(ctx context.Context, notificationID, userID string) error {
	result := r.db.WithContext(ctx).
		Model(&model.Notification{}).
		Where("id = ? AND user_id = ?", notificationID, userID).
		Update("is_read", true)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return common.ErrNotFound
	}
	return nil
}

func (r *notificationRepository) MarkAllRead(ctx context.Context, userID, companyID string) error {
	return r.db.WithContext(ctx).
		Model(&model.Notification{}).
		Where("user_id = ? AND company_id = ? AND is_read = false", userID, companyID).
		Update("is_read", true).Error
}

func (r *notificationRepository) CountUnread(ctx context.Context, userID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Notification{}).
		Where("user_id = ? AND is_read = false", userID).
		Count(&count).Error
	return count, err
}
