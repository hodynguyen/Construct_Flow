package update_task_status

import (
	"context"

	"github.com/hodynguyen/construct-flow/apps/task-service/common"
	"github.com/hodynguyen/construct-flow/apps/task-service/domain"
	"github.com/hodynguyen/construct-flow/apps/task-service/entity/dto"
)

// allowedTransitions defines the valid task status state machine.
// Key = current status, Value = set of allowed next statuses.
var allowedTransitions = map[string]map[string]bool{
	"todo":        {"in_progress": true},
	"in_progress": {"done": true, "blocked": true},
	"blocked":     {"in_progress": true},
	"done":        {"in_progress": true}, // manager/admin only — see managerOnlyTransitions
}

// managerOnlyTransitions requires role=manager or role=admin.
// Used for the "rework" scenario: a completed task sent back for revision.
var managerOnlyTransitions = map[string]map[string]bool{
	"done": {"in_progress": true},
}

// TaskStatusChangedPayload is the event body published to RabbitMQ.
type TaskStatusChangedPayload struct {
	TaskID    string `json:"task_id"`
	TaskTitle string `json:"task_title"`
	ProjectID string `json:"project_id"`
	CompanyID string `json:"company_id"`
	OldStatus string `json:"old_status"`
	NewStatus string `json:"new_status"`
	ChangedBy string `json:"changed_by"`
}

type UseCase struct {
	taskRepo  domain.TaskRepository
	publisher domain.EventPublisher
}

func New(taskRepo domain.TaskRepository, publisher domain.EventPublisher) *UseCase {
	return &UseCase{taskRepo: taskRepo, publisher: publisher}
}

func (uc *UseCase) Execute(ctx context.Context, req dto.UpdateTaskStatusRequest) (*dto.TaskResponse, error) {
	if req.TaskID == "" || req.CompanyID == "" || req.NewStatus == "" || req.ChangedBy == "" {
		return nil, common.ErrInvalidInput
	}

	task, err := uc.taskRepo.FindByID(ctx, req.CompanyID, req.TaskID)
	if err != nil {
		return nil, err
	}

	oldStatus := task.Status

	// Validate transition exists in state machine
	if !allowedTransitions[oldStatus][req.NewStatus] {
		return nil, common.ErrInvalidStatusTransition
	}

	// Some transitions are restricted to manager/admin role
	if managerOnlyTransitions[oldStatus][req.NewStatus] {
		if req.Role != "manager" && req.Role != "admin" {
			return nil, common.ErrForbidden
		}
	}

	task.Status = req.NewStatus
	if err := uc.taskRepo.Update(ctx, task); err != nil {
		return nil, err
	}

	// Publish event — non-fatal (status change succeeds even if notification delivery fails)
	_ = uc.publisher.Publish(ctx, "task.status_changed", TaskStatusChangedPayload{
		TaskID:    task.ID,
		TaskTitle: task.Title,
		ProjectID: task.ProjectID,
		CompanyID: task.CompanyID,
		OldStatus: oldStatus,
		NewStatus: req.NewStatus,
		ChangedBy: req.ChangedBy,
	})

	return dto.ToTaskResponse(task), nil
}
