package model

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

type Company struct {
	ID        string    `gorm:"type:uuid;primaryKey"`
	Name      string    `gorm:"not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (Company) TableName() string { return "companies" }

func (c *Company) BeforeCreate(_ *gorm.DB) error {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	return nil
}
