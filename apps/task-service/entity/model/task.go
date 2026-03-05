package model

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

type Task struct {
	ID          string         `gorm:"type:uuid;primaryKey"`
	ProjectID   string         `gorm:"type:uuid;not null"`
	CompanyID   string         `gorm:"type:uuid;not null"` // denormalized for tenant isolation
	Title       string         `gorm:"not null"`
	Description string
	Status      string         `gorm:"not null;default:todo"`   // todo | in_progress | done | blocked
	Priority    string         `gorm:"not null;default:medium"` // low | medium | high | critical
	AssignedTo  *string        `gorm:"type:uuid"`               // nil = unassigned
	CreatedBy   string         `gorm:"type:uuid;not null"`
	DueDate     *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt `gorm:"index"`
}

func (Task) TableName() string { return "tasks" }

func (t *Task) BeforeCreate(_ *gorm.DB) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	return nil
}
