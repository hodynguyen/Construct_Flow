package dto

import "time"

type TaskResponse struct {
	ID          string
	ProjectID   string
	CompanyID   string
	Title       string
	Description string
	Status      string
	Priority    string
	AssignedTo  string
	CreatedBy   string
	DueDate     string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ProjectResponse struct {
	ID          string
	CompanyID   string
	Name        string
	Description string
	Status      string
	StartDate   string
	EndDate     string
	CreatedBy   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
