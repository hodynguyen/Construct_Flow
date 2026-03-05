package assign_task

import (
	"context"
	"fmt"
	"time"

	"github.com/hodynguyen/construct-flow/apps/task-service/common"
	"github.com/hodynguyen/construct-flow/apps/task-service/domain"
	"github.com/hodynguyen/construct-flow/apps/task-service/entity/dto"
)

const lockTTL = 5 * time.Second

// TaskAssignedPayload is the event body published to RabbitMQ after a successful assignment.
type TaskAssignedPayload struct {
	TaskID     string `json:"task_id"`
	TaskTitle  string `json:"task_title"`
	ProjectID  string `json:"project_id"`
	CompanyID  string `json:"company_id"`
	AssignedTo string `json:"assigned_to"`
	AssignedBy string `json:"assigned_by"`
}

type UseCase struct {
	taskRepo  domain.TaskRepository
	userRepo  domain.UserRepository
	publisher domain.EventPublisher
	locker    domain.LockClient
}

func New(
	taskRepo domain.TaskRepository,
	userRepo domain.UserRepository,
	publisher domain.EventPublisher,
	locker domain.LockClient,
) *UseCase {
	return &UseCase{
		taskRepo:  taskRepo,
		userRepo:  userRepo,
		publisher: publisher,
		locker:    locker,
	}
}

func (uc *UseCase) Execute(ctx context.Context, req dto.AssignTaskRequest) (*dto.TaskResponse, error) {
	if req.TaskID == "" || req.CompanyID == "" || req.AssignedTo == "" || req.AssignedBy == "" {
		return nil, common.ErrInvalidInput
	}

	// 1. Acquire distributed lock — prevents concurrent double-assignment of the same task.
	//    Key is task-scoped so parallel assignments to different tasks are unaffected.
	lockKey := fmt.Sprintf("lock:task:assign:%s", req.TaskID)
	acquired, err := uc.locker.SetNX(ctx, lockKey, req.AssignedBy, lockTTL)
	if err != nil {
		return nil, fmt.Errorf("acquiring lock: %w", err)
	}
	if !acquired {
		return nil, common.ErrTaskLocked
	}
	// Release lock when the use case exits (success or failure).
	// Note: if the 5s TTL expires before we reach here, another caller may have acquired
	// the lock and our Del would remove their lock. Production fix: Lua CAS-delete.
	defer uc.locker.Del(ctx, lockKey) //nolint:errcheck

	// 2. Load task — also enforces company_id tenant scope (cross-tenant access impossible)
	task, err := uc.taskRepo.FindByID(ctx, req.CompanyID, req.TaskID)
	if err != nil {
		return nil, err
	}

	// 3. Validate that the assignee exists in the same company (cross-tenant protection)
	if _, err := uc.userRepo.FindByID(ctx, req.CompanyID, req.AssignedTo); err != nil {
		return nil, common.ErrUserNotFound
	}

	// 4. Perform the assignment and persist
	task.AssignedTo = &req.AssignedTo
	if err := uc.taskRepo.Update(ctx, task); err != nil {
		return nil, err
	}

	// 5. Publish domain event for async notification delivery.
	//    Non-fatal: if RabbitMQ is unavailable the task is still correctly assigned.
	//    Production fix: transactional outbox pattern for guaranteed delivery.
	_ = uc.publisher.Publish(ctx, "task.assigned", TaskAssignedPayload{
		TaskID:     task.ID,
		TaskTitle:  task.Title,
		ProjectID:  task.ProjectID,
		CompanyID:  task.CompanyID,
		AssignedTo: req.AssignedTo,
		AssignedBy: req.AssignedBy,
	})

	return dto.ToTaskResponse(task), nil
}
