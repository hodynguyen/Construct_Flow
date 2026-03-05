package model

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

type User struct {
	ID        string         `gorm:"type:uuid;primaryKey"`
	CompanyID string         `gorm:"type:uuid;not null"`
	Email     string         `gorm:"not null"`
	Name      string         `gorm:"not null"`
	Password  string         `gorm:"not null"` // bcrypt hash (cost=12)
	Role      string         `gorm:"not null;default:worker"` // admin | manager | worker
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (User) TableName() string { return "users" }

func (u *User) BeforeCreate(_ *gorm.DB) error {
	if u.ID == "" {
		u.ID = uuid.New().String()
	}
	return nil
}
