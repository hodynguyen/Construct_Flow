package create_task_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/hodynguyen/construct-flow/apps/task-service/common"
	"github.com/hodynguyen/construct-flow/apps/task-service/entity/dto"
	"github.com/hodynguyen/construct-flow/apps/task-service/entity/model"
	"github.com/hodynguyen/construct-flow/apps/task-service/mock"
	"github.com/hodynguyen/construct-flow/apps/task-service/use-case/create_task"
)

func TestCreateTask(t *testing.T) {
	const (
		companyID = "company-1"
		projectID = "project-1"
		managerID = "manager-1"
	)

	baseProject := &model.Project{
		ID:        projectID,
		CompanyID: companyID,
		Name:      "Building A",
		Status:    "active",
	}

	tests := []struct {
		name    string
		req     dto.CreateTaskRequest
		setup   func(tr *mock.MockTaskRepository, pr *mock.MockProjectRepository)
		wantErr error
		check   func(t *testing.T, resp *dto.TaskResponse)
	}{
		{
			name: "success — creates task with defaults",
			req: dto.CreateTaskRequest{
				CompanyID: companyID,
				ProjectID: projectID,
				CreatedBy: managerID,
				Title:     "Install electrical wiring - Floor 3",
				Priority:  "high",
				DueDate:   "2026-03-15",
			},
			setup: func(tr *mock.MockTaskRepository, pr *mock.MockProjectRepository) {
				pr.EXPECT().FindByID(gomock.Any(), companyID, projectID).Return(baseProject, nil)
				tr.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
			},
			check: func(t *testing.T, resp *dto.TaskResponse) {
				assert.Equal(t, "todo", resp.Status)
				assert.Equal(t, "high", resp.Priority)
				assert.Equal(t, "2026-03-15", resp.DueDate)
				assert.Equal(t, companyID, resp.CompanyID)
			},
		},
		{
			name: "success — defaults priority to medium when not specified",
			req: dto.CreateTaskRequest{
				CompanyID: companyID,
				ProjectID: projectID,
				CreatedBy: managerID,
				Title:     "Paint walls",
			},
			setup: func(tr *mock.MockTaskRepository, pr *mock.MockProjectRepository) {
				pr.EXPECT().FindByID(gomock.Any(), companyID, projectID).Return(baseProject, nil)
				tr.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
			},
			check: func(t *testing.T, resp *dto.TaskResponse) {
				assert.Equal(t, "medium", resp.Priority)
			},
		},
		{
			name:    "invalid_input — missing title",
			req:     dto.CreateTaskRequest{CompanyID: companyID, ProjectID: projectID, CreatedBy: managerID},
			setup:   func(tr *mock.MockTaskRepository, pr *mock.MockProjectRepository) {},
			wantErr: common.ErrInvalidInput,
		},
		{
			name:    "invalid_input — missing company_id",
			req:     dto.CreateTaskRequest{ProjectID: projectID, CreatedBy: managerID, Title: "Task"},
			setup:   func(tr *mock.MockTaskRepository, pr *mock.MockProjectRepository) {},
			wantErr: common.ErrInvalidInput,
		},
		{
			name: "project_not_found — tenant isolation check",
			req: dto.CreateTaskRequest{
				CompanyID: companyID,
				ProjectID: "other-company-project",
				CreatedBy: managerID,
				Title:     "Sneaky task",
			},
			setup: func(tr *mock.MockTaskRepository, pr *mock.MockProjectRepository) {
				pr.EXPECT().FindByID(gomock.Any(), companyID, "other-company-project").Return(nil, common.ErrNotFound)
			},
			wantErr: common.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			tr := mock.NewMockTaskRepository(ctrl)
			pr := mock.NewMockProjectRepository(ctrl)
			tt.setup(tr, pr)

			uc := create_task.New(tr, pr)
			resp, err := uc.Execute(context.Background(), tt.req)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				if tt.check != nil {
					tt.check(t, resp)
				}
			}
		})
	}
}
