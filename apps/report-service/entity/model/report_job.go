package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ReportJob struct {
	ID          string  `gorm:"type:uuid;primaryKey"`
	CompanyID   string  `gorm:"type:uuid;not null;index"`
	RequestedBy string  `gorm:"type:uuid;not null"`
	Type        string  `gorm:"size:50;not null"`   // weekly_progress | monthly_summary | task_overdue
	Params      string  `gorm:"type:jsonb"`         // JSON: { project_id, week, ... }
	Status      string  `gorm:"size:20;not null;default:'queued'"` // queued | processing | ready | failed
	S3Key       string  `gorm:"size:1000"`
	DownloadURL string  `gorm:"size:2000"`
	ErrorMsg    *string
	CreatedAt   time.Time
	CompletedAt *time.Time
}

func (r *ReportJob) BeforeCreate(_ *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.NewString()
	}
	return nil
}
