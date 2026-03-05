package model

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

type Project struct {
	ID          string         `gorm:"type:uuid;primaryKey"`
	CompanyID   string         `gorm:"type:uuid;not null;index"`
	Name        string         `gorm:"not null"`
	Description string
	Status      string         `gorm:"not null;default:active"` // active | completed | archived
	StartDate   *time.Time
	EndDate     *time.Time
	CreatedBy   string         `gorm:"type:uuid"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt `gorm:"index"`
}

func (Project) TableName() string { return "projects" }

func (p *Project) BeforeCreate(_ *gorm.DB) error {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	return nil
}
