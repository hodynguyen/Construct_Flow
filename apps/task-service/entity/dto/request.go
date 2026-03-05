package dto

type CreateTaskRequest struct {
	CompanyID   string
	ProjectID   string
	CreatedBy   string
	Title       string
	Description string
	Priority    string // low | medium | high | critical (default: medium)
	AssignedTo  string // optional
	DueDate     string // YYYY-MM-DD, optional
}

type AssignTaskRequest struct {
	CompanyID  string
	TaskID     string
	AssignedTo string // user_id of the assignee
	AssignedBy string // user_id of the manager performing the action
}

type UpdateTaskStatusRequest struct {
	CompanyID string
	TaskID    string
	NewStatus string // todo | in_progress | done | blocked
	ChangedBy string
	Role      string // required: manager can do done→in_progress, worker cannot
}

type CreateProjectRequest struct {
	CompanyID   string
	CreatedBy   string
	Name        string
	Description string
	StartDate   string // YYYY-MM-DD, optional
	EndDate     string // YYYY-MM-DD, optional
}
