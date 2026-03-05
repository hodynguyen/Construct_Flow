package dto

import "github.com/hodynguyen/construct-flow/apps/task-service/entity/model"

func ToTaskResponse(t *model.Task) *TaskResponse {
	resp := &TaskResponse{
		ID:          t.ID,
		ProjectID:   t.ProjectID,
		CompanyID:   t.CompanyID,
		Title:       t.Title,
		Description: t.Description,
		Status:      t.Status,
		Priority:    t.Priority,
		CreatedBy:   t.CreatedBy,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
	if t.AssignedTo != nil {
		resp.AssignedTo = *t.AssignedTo
	}
	if t.DueDate != nil {
		resp.DueDate = t.DueDate.Format("2006-01-02")
	}
	return resp
}

func ToProjectResponse(p *model.Project) *ProjectResponse {
	resp := &ProjectResponse{
		ID:          p.ID,
		CompanyID:   p.CompanyID,
		Name:        p.Name,
		Description: p.Description,
		Status:      p.Status,
		CreatedBy:   p.CreatedBy,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
	if p.StartDate != nil {
		resp.StartDate = p.StartDate.Format("2006-01-02")
	}
	if p.EndDate != nil {
		resp.EndDate = p.EndDate.Format("2006-01-02")
	}
	return resp
}
