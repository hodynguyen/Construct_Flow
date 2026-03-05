package model

import "time"

// Company is a read-only reference model — created/owned by user-service.
// task-service reads it only for FK resolution via shared PostgreSQL.
type Company struct {
	ID        string    `gorm:"type:uuid;primaryKey"`
	Name      string    `gorm:"not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (Company) TableName() string { return "companies" }
