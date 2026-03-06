package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type File struct {
	ID          string    `gorm:"type:uuid;primaryKey"`
	CompanyID   string    `gorm:"type:uuid;not null;index"`
	ProjectID   *string   `gorm:"type:uuid;index"`
	TaskID      *string   `gorm:"type:uuid;index"`
	UploadedBy  string    `gorm:"type:uuid;not null"`
	Name        string    `gorm:"not null"`
	S3Key       string    `gorm:"not null"`
	S3Bucket    string    `gorm:"not null"`
	SizeBytes   int64
	MimeType    string `gorm:"size:100"`
	StorageTier string `gorm:"size:20;not null;default:'standard'"` // standard | standard_ia | glacier
	Status      string `gorm:"size:20;not null;default:'pending'"`  // pending | active | deleted
	CreatedAt   time.Time
	DeletedAt   gorm.DeletedAt `gorm:"index"`
}

func (f *File) BeforeCreate(_ *gorm.DB) error {
	if f.ID == "" {
		f.ID = uuid.NewString()
	}
	return nil
}
