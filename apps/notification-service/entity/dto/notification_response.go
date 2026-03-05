package dto

import "time"

// NotificationResponse is the read model returned to callers.
type NotificationResponse struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	CompanyID string    `json:"company_id"`
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	IsRead    bool      `json:"is_read"`
	Metadata  string    `json:"metadata"`
	CreatedAt time.Time `json:"created_at"`
}
