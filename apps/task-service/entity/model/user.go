package model

import (
	"gorm.io/gorm"
	"time"
)

// User is a read-only reference model in task-service.
// Owned by user-service; task-service reads it to validate assignee company membership.
type User struct {
	ID        string         `gorm:"type:uuid;primaryKey"`
	CompanyID string         `gorm:"type:uuid;not null"`
	Email     string         `gorm:"not null"`
	Name      string         `gorm:"not null"`
	Role      string         `gorm:"not null;default:worker"` // admin | manager | worker
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (User) TableName() string { return "users" }
