package model

import "time"

// AuditLog is append-only — never updated or deleted.
// Table is range-partitioned by occurred_at (monthly).
type AuditLog struct {
	ID          string    `gorm:"type:uuid;primaryKey"`
	CompanyID   string    `gorm:"type:uuid;not null;index"`
	UserID      string    `gorm:"type:uuid;not null"`
	Action      string    `gorm:"size:100;not null;index"` // task.assigned | file.uploaded | user.login
	Resource    string    `gorm:"size:50;not null;index"`  // task | file | user
	ResourceID  string    `gorm:"type:uuid;not null;index"`
	BeforeState string    `gorm:"type:jsonb"`
	AfterState  string    `gorm:"type:jsonb"`
	IPAddress   string    `gorm:"size:45"`
	OccurredAt  time.Time `gorm:"not null;index"`
}
