package controller

import (
	"context"
	"errors"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/hodynguyen/construct-flow/apps/task-service/common"
	"github.com/hodynguyen/construct-flow/apps/task-service/domain"
	"github.com/hodynguyen/construct-flow/apps/task-service/entity/dto"
	"github.com/hodynguyen/construct-flow/apps/task-service/use-case/assign_task"
	"github.com/hodynguyen/construct-flow/apps/task-service/use-case/create_project"
	"github.com/hodynguyen/construct-flow/apps/task-service/use-case/create_task"
	"github.com/hodynguyen/construct-flow/apps/task-service/use-case/update_task_status"
	taskv1 "github.com/hodynguyen/construct-flow/gen/go/proto/task_service/v1"
)

// TaskController implements the gRPC TaskServiceServer.
type TaskController struct {
	taskv1.UnimplementedTaskServiceServer
	projectRepo      domain.ProjectRepository
	taskRepo         domain.TaskRepository
	createProjectUC  *create_project.UseCase
	createTaskUC     *create_task.UseCase
	assignTaskUC     *assign_task.UseCase
	updateStatusUC   *update_task_status.UseCase
}

func NewTaskController(
	projectRepo domain.ProjectRepository,
	taskRepo domain.TaskRepository,
	createProjectUC *create_project.UseCase,
	createTaskUC *create_task.UseCase,
	assignTaskUC *assign_task.UseCase,
	updateStatusUC *update_task_status.UseCase,
) *TaskController {
	return &TaskController{
		projectRepo:     projectRepo,
		taskRepo:        taskRepo,
		createProjectUC: createProjectUC,
		createTaskUC:    createTaskUC,
		assignTaskUC:    assignTaskUC,
		updateStatusUC:  updateStatusUC,
	}
}

// ── Project RPCs ────────────────────────────────────────────────────────────

func (c *TaskController) CreateProject(ctx context.Context, req *taskv1.CreateProjectRequest) (*taskv1.ProjectResponse, error) {
	resp, err := c.createProjectUC.Execute(ctx, dto.CreateProjectRequest{
		CompanyID:   req.CompanyId,
		CreatedBy:   req.CreatedBy,
		Name:        req.Name,
		Description: req.Description,
		StartDate:   req.StartDate,
		EndDate:     req.EndDate,
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &taskv1.ProjectResponse{Project: toProtoProject(resp)}, nil
}

func (c *TaskController) GetProject(ctx context.Context, req *taskv1.GetProjectRequest) (*taskv1.ProjectResponse, error) {
	project, err := c.projectRepo.FindByID(ctx, req.CompanyId, req.Id)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &taskv1.ProjectResponse{Project: toProtoProject(dto.ToProjectResponse(project))}, nil
}

func (c *TaskController) ListProjects(ctx context.Context, req *taskv1.ListProjectsRequest) (*taskv1.ListProjectsResponse, error) {
	page, pageSize := normalizePage(int(req.Page), int(req.PageSize))
	projects, total, err := c.projectRepo.ListByCompany(ctx, req.CompanyId,
		domain.ListProjectFilter{Status: req.Status}, page, pageSize)
	if err != nil {
		return nil, toGRPCError(err)
	}
	var protoProjects []*taskv1.Project
	for i := range projects {
		protoProjects = append(protoProjects, toProtoProject(dto.ToProjectResponse(&projects[i])))
	}
	return &taskv1.ListProjectsResponse{
		Projects: protoProjects,
		Total:    int32(total),
		Page:     int32(page),
		PageSize: int32(pageSize),
	}, nil
}

func (c *TaskController) UpdateProject(ctx context.Context, req *taskv1.UpdateProjectRequest) (*taskv1.ProjectResponse, error) {
	project, err := c.projectRepo.FindByID(ctx, req.CompanyId, req.Id)
	if err != nil {
		return nil, toGRPCError(err)
	}
	if req.Name != "" {
		project.Name = req.Name
	}
	if req.Description != "" {
		project.Description = req.Description
	}
	if req.Status != "" {
		project.Status = req.Status
	}
	if req.EndDate != "" {
		t, err := time.Parse("2006-01-02", req.EndDate)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid end_date format")
		}
		project.EndDate = &t
	}
	if err := c.projectRepo.Update(ctx, project); err != nil {
		return nil, toGRPCError(err)
	}
	return &taskv1.ProjectResponse{Project: toProtoProject(dto.ToProjectResponse(project))}, nil
}

func (c *TaskController) DeleteProject(ctx context.Context, req *taskv1.DeleteProjectRequest) (*emptypb.Empty, error) {
	if err := c.projectRepo.Delete(ctx, req.CompanyId, req.Id); err != nil {
		return nil, toGRPCError(err)
	}
	return &emptypb.Empty{}, nil
}

// ── Task RPCs ───────────────────────────────────────────────────────────────

func (c *TaskController) CreateTask(ctx context.Context, req *taskv1.CreateTaskRequest) (*taskv1.TaskResponse, error) {
	resp, err := c.createTaskUC.Execute(ctx, dto.CreateTaskRequest{
		CompanyID:   req.CompanyId,
		ProjectID:   req.ProjectId,
		CreatedBy:   req.CreatedBy,
		Title:       req.Title,
		Description: req.Description,
		Priority:    req.Priority,
		DueDate:     req.DueDate,
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &taskv1.TaskResponse{Task: toProtoTask(resp)}, nil
}

func (c *TaskController) GetTask(ctx context.Context, req *taskv1.GetTaskRequest) (*taskv1.TaskResponse, error) {
	task, err := c.taskRepo.FindByID(ctx, req.CompanyId, req.Id)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &taskv1.TaskResponse{Task: toProtoTask(dto.ToTaskResponse(task))}, nil
}

func (c *TaskController) ListTasks(ctx context.Context, req *taskv1.ListTasksRequest) (*taskv1.ListTasksResponse, error) {
	page, pageSize := normalizePage(int(req.Page), int(req.PageSize))
	tasks, total, err := c.taskRepo.ListByProject(ctx, req.CompanyId, req.ProjectId,
		domain.ListTaskFilter{
			Status:     req.Status,
			AssignedTo: req.AssignedTo,
			Priority:   req.Priority,
		}, page, pageSize)
	if err != nil {
		return nil, toGRPCError(err)
	}
	var protoTasks []*taskv1.Task
	for i := range tasks {
		protoTasks = append(protoTasks, toProtoTask(dto.ToTaskResponse(&tasks[i])))
	}
	return &taskv1.ListTasksResponse{
		Tasks:    protoTasks,
		Total:    int32(total),
		Page:     int32(page),
		PageSize: int32(pageSize),
	}, nil
}

func (c *TaskController) UpdateTask(ctx context.Context, req *taskv1.UpdateTaskRequest) (*taskv1.TaskResponse, error) {
	task, err := c.taskRepo.FindByID(ctx, req.CompanyId, req.Id)
	if err != nil {
		return nil, toGRPCError(err)
	}
	if req.Title != "" {
		task.Title = req.Title
	}
	if req.Description != "" {
		task.Description = req.Description
	}
	if req.Priority != "" {
		task.Priority = req.Priority
	}
	if req.DueDate != "" {
		t, err := time.Parse("2006-01-02", req.DueDate)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid due_date format")
		}
		task.DueDate = &t
	}
	if err := c.taskRepo.Update(ctx, task); err != nil {
		return nil, toGRPCError(err)
	}
	return &taskv1.TaskResponse{Task: toProtoTask(dto.ToTaskResponse(task))}, nil
}

func (c *TaskController) DeleteTask(ctx context.Context, req *taskv1.DeleteTaskRequest) (*emptypb.Empty, error) {
	if err := c.taskRepo.Delete(ctx, req.CompanyId, req.Id); err != nil {
		return nil, toGRPCError(err)
	}
	return &emptypb.Empty{}, nil
}

func (c *TaskController) AssignTask(ctx context.Context, req *taskv1.AssignTaskRequest) (*taskv1.TaskResponse, error) {
	resp, err := c.assignTaskUC.Execute(ctx, dto.AssignTaskRequest{
		CompanyID:  req.CompanyId,
		TaskID:     req.TaskId,
		AssignedTo: req.AssignedTo,
		AssignedBy: req.AssignedBy,
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &taskv1.TaskResponse{Task: toProtoTask(resp)}, nil
}

func (c *TaskController) UpdateTaskStatus(ctx context.Context, req *taskv1.UpdateTaskStatusRequest) (*taskv1.TaskResponse, error) {
	resp, err := c.updateStatusUC.Execute(ctx, dto.UpdateTaskStatusRequest{
		CompanyID: req.CompanyId,
		TaskID:    req.TaskId,
		NewStatus: req.NewStatus,
		ChangedBy: req.ChangedBy,
		Role:      req.Role,
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &taskv1.TaskResponse{Task: toProtoTask(resp)}, nil
}

// ── Mappers ─────────────────────────────────────────────────────────────────

func toProtoTask(t *dto.TaskResponse) *taskv1.Task {
	return &taskv1.Task{
		Id:          t.ID,
		ProjectId:   t.ProjectID,
		CompanyId:   t.CompanyID,
		Title:       t.Title,
		Description: t.Description,
		Status:      t.Status,
		Priority:    t.Priority,
		AssignedTo:  t.AssignedTo,
		CreatedBy:   t.CreatedBy,
		DueDate:     t.DueDate,
		CreatedAt:   timestamppb.New(t.CreatedAt),
		UpdatedAt:   timestamppb.New(t.UpdatedAt),
	}
}

func toProtoProject(p *dto.ProjectResponse) *taskv1.Project {
	return &taskv1.Project{
		Id:          p.ID,
		CompanyId:   p.CompanyID,
		Name:        p.Name,
		Description: p.Description,
		Status:      p.Status,
		StartDate:   p.StartDate,
		EndDate:     p.EndDate,
		CreatedBy:   p.CreatedBy,
		CreatedAt:   timestamppb.New(p.CreatedAt),
		UpdatedAt:   timestamppb.New(p.UpdatedAt),
	}
}

func toGRPCError(err error) error {
	switch {
	case errors.Is(err, common.ErrNotFound), errors.Is(err, common.ErrUserNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, common.ErrInvalidInput), errors.Is(err, common.ErrInvalidStatusTransition):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, common.ErrForbidden):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, common.ErrTaskLocked):
		return status.Error(codes.Aborted, err.Error())
	default:
		return status.Error(codes.Internal, "internal error")
	}
}

func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return page, pageSize
}

