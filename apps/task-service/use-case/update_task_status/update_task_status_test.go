package update_task_status_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/hodynguyen/construct-flow/apps/task-service/common"
	"github.com/hodynguyen/construct-flow/apps/task-service/entity/dto"
	"github.com/hodynguyen/construct-flow/apps/task-service/entity/model"
	"github.com/hodynguyen/construct-flow/apps/task-service/mock"
	"github.com/hodynguyen/construct-flow/apps/task-service/use-case/update_task_status"
)

func TestUpdateTaskStatus(t *testing.T) {
	const (
		companyID = "company-1"
		taskID    = "task-1"
		workerID  = "worker-1"
		managerID = "manager-1"
	)

	taskWithStatus := func(status string) *model.Task {
		return &model.Task{
			ID:        taskID,
			CompanyID: companyID,
			ProjectID: "project-1",
			Title:     "Install wiring",
			Status:    status,
		}
	}

	tests := []struct {
		name       string
		req        dto.UpdateTaskStatusRequest
		fromStatus string // task's current status in DB
		setup      func(tr *mock.MockTaskRepository, pub *mock.MockEventPublisher)
		wantErr    error
		wantStatus string
	}{
		// ── Valid transitions ────────────────────────────────────────────────
		{
			name:       "todo → in_progress (worker allowed)",
			fromStatus: "todo",
			req:        dto.UpdateTaskStatusRequest{CompanyID: companyID, TaskID: taskID, NewStatus: "in_progress", ChangedBy: workerID, Role: "worker"},
			setup: func(tr *mock.MockTaskRepository, pub *mock.MockEventPublisher) {
				tr.EXPECT().FindByID(gomock.Any(), companyID, taskID).Return(taskWithStatus("todo"), nil)
				tr.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
				pub.EXPECT().Publish(gomock.Any(), "task.status_changed", gomock.Any()).Return(nil)
			},
			wantStatus: "in_progress",
		},
		{
			name:       "in_progress → done (worker allowed)",
			fromStatus: "in_progress",
			req:        dto.UpdateTaskStatusRequest{CompanyID: companyID, TaskID: taskID, NewStatus: "done", ChangedBy: workerID, Role: "worker"},
			setup: func(tr *mock.MockTaskRepository, pub *mock.MockEventPublisher) {
				tr.EXPECT().FindByID(gomock.Any(), companyID, taskID).Return(taskWithStatus("in_progress"), nil)
				tr.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
				pub.EXPECT().Publish(gomock.Any(), "task.status_changed", gomock.Any()).Return(nil)
			},
			wantStatus: "done",
		},
		{
			name:       "in_progress → blocked (worker allowed)",
			fromStatus: "in_progress",
			req:        dto.UpdateTaskStatusRequest{CompanyID: companyID, TaskID: taskID, NewStatus: "blocked", ChangedBy: workerID, Role: "worker"},
			setup: func(tr *mock.MockTaskRepository, pub *mock.MockEventPublisher) {
				tr.EXPECT().FindByID(gomock.Any(), companyID, taskID).Return(taskWithStatus("in_progress"), nil)
				tr.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
				pub.EXPECT().Publish(gomock.Any(), "task.status_changed", gomock.Any()).Return(nil)
			},
			wantStatus: "blocked",
		},
		{
			name:       "blocked → in_progress (worker allowed)",
			fromStatus: "blocked",
			req:        dto.UpdateTaskStatusRequest{CompanyID: companyID, TaskID: taskID, NewStatus: "in_progress", ChangedBy: workerID, Role: "worker"},
			setup: func(tr *mock.MockTaskRepository, pub *mock.MockEventPublisher) {
				tr.EXPECT().FindByID(gomock.Any(), companyID, taskID).Return(taskWithStatus("blocked"), nil)
				tr.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
				pub.EXPECT().Publish(gomock.Any(), "task.status_changed", gomock.Any()).Return(nil)
			},
			wantStatus: "in_progress",
		},
		{
			name:       "done → in_progress (manager allowed — rework)",
			fromStatus: "done",
			req:        dto.UpdateTaskStatusRequest{CompanyID: companyID, TaskID: taskID, NewStatus: "in_progress", ChangedBy: managerID, Role: "manager"},
			setup: func(tr *mock.MockTaskRepository, pub *mock.MockEventPublisher) {
				tr.EXPECT().FindByID(gomock.Any(), companyID, taskID).Return(taskWithStatus("done"), nil)
				tr.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
				pub.EXPECT().Publish(gomock.Any(), "task.status_changed", gomock.Any()).Return(nil)
			},
			wantStatus: "in_progress",
		},

		// ── Forbidden transitions ─────────────────────────────────────────────
		{
			name:       "done → in_progress (worker FORBIDDEN — rework is manager-only)",
			fromStatus: "done",
			req:        dto.UpdateTaskStatusRequest{CompanyID: companyID, TaskID: taskID, NewStatus: "in_progress", ChangedBy: workerID, Role: "worker"},
			setup: func(tr *mock.MockTaskRepository, pub *mock.MockEventPublisher) {
				tr.EXPECT().FindByID(gomock.Any(), companyID, taskID).Return(taskWithStatus("done"), nil)
			},
			wantErr: common.ErrForbidden,
		},

		// ── Invalid state machine transitions ─────────────────────────────────
		{
			name:       "done → todo (INVALID — not in state machine)",
			fromStatus: "done",
			req:        dto.UpdateTaskStatusRequest{CompanyID: companyID, TaskID: taskID, NewStatus: "todo", ChangedBy: managerID, Role: "manager"},
			setup: func(tr *mock.MockTaskRepository, pub *mock.MockEventPublisher) {
				tr.EXPECT().FindByID(gomock.Any(), companyID, taskID).Return(taskWithStatus("done"), nil)
			},
			wantErr: common.ErrInvalidStatusTransition,
		},
		{
			name:       "todo → done (INVALID — must go through in_progress first)",
			fromStatus: "todo",
			req:        dto.UpdateTaskStatusRequest{CompanyID: companyID, TaskID: taskID, NewStatus: "done", ChangedBy: workerID, Role: "worker"},
			setup: func(tr *mock.MockTaskRepository, pub *mock.MockEventPublisher) {
				tr.EXPECT().FindByID(gomock.Any(), companyID, taskID).Return(taskWithStatus("todo"), nil)
			},
			wantErr: common.ErrInvalidStatusTransition,
		},
		{
			name:       "todo → blocked (INVALID)",
			fromStatus: "todo",
			req:        dto.UpdateTaskStatusRequest{CompanyID: companyID, TaskID: taskID, NewStatus: "blocked", ChangedBy: workerID, Role: "worker"},
			setup: func(tr *mock.MockTaskRepository, pub *mock.MockEventPublisher) {
				tr.EXPECT().FindByID(gomock.Any(), companyID, taskID).Return(taskWithStatus("todo"), nil)
			},
			wantErr: common.ErrInvalidStatusTransition,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			tr := mock.NewMockTaskRepository(ctrl)
			pub := mock.NewMockEventPublisher(ctrl)
			tt.setup(tr, pub)

			uc := update_task_status.New(tr, pub)
			resp, err := uc.Execute(context.Background(), tt.req)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, tt.wantStatus, resp.Status)
			}
		})
	}
}
