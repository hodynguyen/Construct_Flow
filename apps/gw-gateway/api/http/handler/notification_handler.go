package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/hodynguyen/construct-flow/apps/gw-gateway/api/http/middleware"
	notifv1 "github.com/hodynguyen/construct-flow/gen/go/proto/notification_service/v1"
)

// NotificationHandler forwards notification operations to notification-service gRPC.
type NotificationHandler struct {
	notifClient notifv1.NotificationServiceClient
}

func NewNotificationHandler(notifClient notifv1.NotificationServiceClient) *NotificationHandler {
	return &NotificationHandler{notifClient: notifClient}
}

// GetNotifications godoc
// @Summary List notifications for the current user
// @Tags notifications
// @Security BearerAuth
// @Router /api/v1/notifications [get]
func (h *NotificationHandler) GetNotifications(c *gin.Context) {
	userID, companyID, _, err := middleware.GetClaims(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	unreadOnly := c.Query("unread_only") == "true"

	resp, err := h.notifClient.GetNotifications(c.Request.Context(), &notifv1.GetNotificationsRequest{
		UserId:    userID,
		CompanyId: companyID,
		UnreadOnly: unreadOnly,
		Page:      int32(page),
		PageSize:  int32(pageSize),
	})
	if err != nil {
		c.JSON(grpcToHTTPStatus(err), gin.H{"error": grpcMessage(err)})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"notifications": resp.Notifications,
		"total":         resp.Total,
		"page":          resp.Page,
		"page_size":     resp.PageSize,
	})
}

// GetUnreadCount godoc
// @Summary Get unread notification count
// @Tags notifications
// @Security BearerAuth
// @Router /api/v1/notifications/unread/count [get]
func (h *NotificationHandler) GetUnreadCount(c *gin.Context) {
	userID, companyID, _, err := middleware.GetClaims(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	resp, err := h.notifClient.GetUnreadCount(c.Request.Context(), &notifv1.GetUnreadCountRequest{
		UserId:    userID,
		CompanyId: companyID,
	})
	if err != nil {
		c.JSON(grpcToHTTPStatus(err), gin.H{"error": grpcMessage(err)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": resp.Count})
}

// MarkAsRead godoc
// @Summary Mark a notification as read
// @Tags notifications
// @Security BearerAuth
// @Router /api/v1/notifications/{id}/read [patch]
func (h *NotificationHandler) MarkAsRead(c *gin.Context) {
	userID, companyID, _, err := middleware.GetClaims(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	_, err = h.notifClient.MarkAsRead(c.Request.Context(), &notifv1.MarkAsReadRequest{
		NotificationId: c.Param("id"),
		UserId:         userID,
		CompanyId:      companyID,
	})
	if err != nil {
		c.JSON(grpcToHTTPStatus(err), gin.H{"error": grpcMessage(err)})
		return
	}
	c.Status(http.StatusNoContent)
}
