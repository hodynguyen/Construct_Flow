package model

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

type Notification struct {
	ID        string    `gorm:"type:uuid;primaryKey"`
	UserID    string    `gorm:"type:uuid;not null"`
	CompanyID string    `gorm:"type:uuid;not null"`
	Type      string    `gorm:"not null"` // task_assigned | status_changed
	Title     string    `gorm:"not null"`
	Message   string
	IsRead    bool      `gorm:"not null;default:false"`
	Metadata  string    `gorm:"type:jsonb"` // JSON string
	CreatedAt time.Time
}

func (Notification) TableName() string { return "notifications" }

func (n *Notification) BeforeCreate(_ *gorm.DB) error {
	if n.ID == "" {
		n.ID = uuid.New().String()
	}
	return nil
}
