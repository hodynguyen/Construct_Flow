package assign_task_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/hodynguyen/construct-flow/apps/task-service/common"
	"github.com/hodynguyen/construct-flow/apps/task-service/entity/dto"
	"github.com/hodynguyen/construct-flow/apps/task-service/entity/model"
	"github.com/hodynguyen/construct-flow/apps/task-service/mock"
	"github.com/hodynguyen/construct-flow/apps/task-service/use-case/assign_task"
)

func TestAssignTask(t *testing.T) {
	const (
		companyID  = "company-1"
		taskID     = "task-1"
		projectID  = "project-1"
		assigneeID = "worker-1"
		managerID  = "manager-1"
		lockKey    = "lock:task:assign:" + taskID
	)

	baseTask := &model.Task{
		ID:        taskID,
		CompanyID: companyID,
		ProjectID: projectID,
		Title:     "Install electrical wiring - Floor 3",
		Status:    "todo",
	}

	baseAssignee := &model.User{
		ID:        assigneeID,
		CompanyID: companyID,
		Role:      "worker",
	}

	validReq := dto.AssignTaskRequest{
		CompanyID:  companyID,
		TaskID:     taskID,
		AssignedTo: assigneeID,
		AssignedBy: managerID,
	}

	tests := []struct {
		name         string
		req          dto.AssignTaskRequest
		setup        func(tr *mock.MockTaskRepository, ur *mock.MockUserRepository, pub *mock.MockEventPublisher, lk *mock.MockLockClient)
		wantErr      error
		wantAssigned string // expected AssignedTo in response
	}{
		{
			name: "success",
			req:  validReq,
			setup: func(tr *mock.MockTaskRepository, ur *mock.MockUserRepository, pub *mock.MockEventPublisher, lk *mock.MockLockClient) {
				lk.EXPECT().SetNX(gomock.Any(), lockKey, managerID, 5*time.Second).Return(true, nil)
				lk.EXPECT().Del(gomock.Any(), lockKey).Return(nil)
				tr.EXPECT().FindByID(gomock.Any(), companyID, taskID).Return(baseTask, nil)
				ur.EXPECT().FindByID(gomock.Any(), companyID, assigneeID).Return(baseAssignee, nil)
				tr.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
				pub.EXPECT().Publish(gomock.Any(), "task.assigned", gomock.Any()).Return(nil)
			},
			wantAssigned: assigneeID,
		},
		{
			name: "lock_already_held — concurrent assignment blocked",
			req:  validReq,
			setup: func(tr *mock.MockTaskRepository, ur *mock.MockUserRepository, pub *mock.MockEventPublisher, lk *mock.MockLockClient) {
				lk.EXPECT().SetNX(gomock.Any(), lockKey, managerID, 5*time.Second).Return(false, nil)
			},
			wantErr: common.ErrTaskLocked,
		},
		{
			name: "task_not_found",
			req:  validReq,
			setup: func(tr *mock.MockTaskRepository, ur *mock.MockUserRepository, pub *mock.MockEventPublisher, lk *mock.MockLockClient) {
				lk.EXPECT().SetNX(gomock.Any(), lockKey, managerID, 5*time.Second).Return(true, nil)
				lk.EXPECT().Del(gomock.Any(), lockKey).Return(nil)
				tr.EXPECT().FindByID(gomock.Any(), companyID, taskID).Return(nil, common.ErrNotFound)
			},
			wantErr: common.ErrNotFound,
		},
		{
			name: "cross_tenant_blocked — assignee from different company",
			req:  validReq,
			setup: func(tr *mock.MockTaskRepository, ur *mock.MockUserRepository, pub *mock.MockEventPublisher, lk *mock.MockLockClient) {
				lk.EXPECT().SetNX(gomock.Any(), lockKey, managerID, 5*time.Second).Return(true, nil)
				lk.EXPECT().Del(gomock.Any(), lockKey).Return(nil)
				tr.EXPECT().FindByID(gomock.Any(), companyID, taskID).Return(baseTask, nil)
				// FindByID with company_id scope returns not found → cross-tenant attempt rejected
				ur.EXPECT().FindByID(gomock.Any(), companyID, assigneeID).Return(nil, common.ErrUserNotFound)
			},
			wantErr: common.ErrUserNotFound,
		},
		{
			name: "publish_fails — task still assigned (notification is secondary)",
			req:  validReq,
			setup: func(tr *mock.MockTaskRepository, ur *mock.MockUserRepository, pub *mock.MockEventPublisher, lk *mock.MockLockClient) {
				lk.EXPECT().SetNX(gomock.Any(), lockKey, managerID, 5*time.Second).Return(true, nil)
				lk.EXPECT().Del(gomock.Any(), lockKey).Return(nil)
				tr.EXPECT().FindByID(gomock.Any(), companyID, taskID).Return(baseTask, nil)
				ur.EXPECT().FindByID(gomock.Any(), companyID, assigneeID).Return(baseAssignee, nil)
				tr.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
				pub.EXPECT().Publish(gomock.Any(), "task.assigned", gomock.Any()).Return(errors.New("rabbitmq unavailable"))
			},
			wantAssigned: assigneeID,
		},
		{
			name: "invalid_input — missing required fields",
			req:  dto.AssignTaskRequest{CompanyID: companyID, TaskID: taskID}, // no AssignedTo/AssignedBy
			setup: func(tr *mock.MockTaskRepository, ur *mock.MockUserRepository, pub *mock.MockEventPublisher, lk *mock.MockLockClient) {
				// no expectations: validation fails before any external call
			},
			wantErr: common.ErrInvalidInput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			tr := mock.NewMockTaskRepository(ctrl)
			ur := mock.NewMockUserRepository(ctrl)
			pub := mock.NewMockEventPublisher(ctrl)
			lk := mock.NewMockLockClient(ctrl)

			tt.setup(tr, ur, pub, lk)

			uc := assign_task.New(tr, ur, pub, lk)
			resp, err := uc.Execute(context.Background(), tt.req)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, tt.wantAssigned, resp.AssignedTo)
			}
		})
	}
}
