package register_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/hodynguyen/construct-flow/apps/user-service/common"
	"github.com/hodynguyen/construct-flow/apps/user-service/entity/dto"
	"github.com/hodynguyen/construct-flow/apps/user-service/entity/model"
	"github.com/hodynguyen/construct-flow/apps/user-service/mock"
	"github.com/hodynguyen/construct-flow/apps/user-service/use-case/register"
)

func TestRegister(t *testing.T) {
	const (
		companyID   = "company-1"
		companyName = "ACME Construction"
	)

	const testCredential = "testcred_abc123"

	validReq := dto.RegisterRequest{
		Email:       "manager@acme.com",
		Name:        "Suzuki Manager",
		Password:    testCredential,
		Role:        "manager",
		CompanyName: companyName,
	}

	baseCompany := &model.Company{
		ID:   companyID,
		Name: companyName,
	}

	tests := []struct {
		name    string
		req     dto.RegisterRequest
		setup   func(ur *mock.MockUserRepository, cr *mock.MockCompanyRepository)
		wantErr error
		check   func(t *testing.T, resp *dto.RegisterResponse)
	}{
		{
			name: "success — creates company and user",
			req:  validReq,
			setup: func(ur *mock.MockUserRepository, cr *mock.MockCompanyRepository) {
				cr.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, c *model.Company) error {
						c.ID = companyID
						return nil
					},
				)
				ur.EXPECT().ExistsByEmail(gomock.Any(), validReq.Email, companyID).Return(false, nil)
				ur.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
			},
			check: func(t *testing.T, resp *dto.RegisterResponse) {
				assert.Equal(t, companyID, resp.CompanyID)
				assert.Equal(t, validReq.Email, resp.User.Email)
				assert.Equal(t, "manager", resp.User.Role)
			},
		},
		{
			name: "success — join existing company",
			req: dto.RegisterRequest{
				Email:     "worker@acme.com",
				Name:      "Tanaka Worker",
				Password:  testCredential,
				Role:      "worker",
				CompanyID: companyID,
			},
			setup: func(ur *mock.MockUserRepository, cr *mock.MockCompanyRepository) {
				cr.EXPECT().FindByID(gomock.Any(), companyID).Return(baseCompany, nil)
				ur.EXPECT().ExistsByEmail(gomock.Any(), "worker@acme.com", companyID).Return(false, nil)
				ur.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
			},
			check: func(t *testing.T, resp *dto.RegisterResponse) {
				assert.Equal(t, companyID, resp.CompanyID)
			},
		},
		{
			name:    "invalid_input — missing email",
			req:     dto.RegisterRequest{Name: "X", Password: testCredential, CompanyName: companyName},
			setup:   func(ur *mock.MockUserRepository, cr *mock.MockCompanyRepository) {},
			wantErr: common.ErrInvalidInput,
		},
		{
			name:    "invalid_input — no company specified",
			req:     dto.RegisterRequest{Email: "x@x.com", Name: "X", Password: "p"},
			setup:   func(ur *mock.MockUserRepository, cr *mock.MockCompanyRepository) {},
			wantErr: common.ErrInvalidInput,
		},
		{
			name: "email_already_exists",
			req:  validReq,
			setup: func(ur *mock.MockUserRepository, cr *mock.MockCompanyRepository) {
				cr.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, c *model.Company) error {
						c.ID = companyID
						return nil
					},
				)
				ur.EXPECT().ExistsByEmail(gomock.Any(), validReq.Email, companyID).Return(true, nil)
			},
			wantErr: common.ErrEmailExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ur := mock.NewMockUserRepository(ctrl)
			cr := mock.NewMockCompanyRepository(ctrl)
			tt.setup(ur, cr)

			uc := register.New(ur, cr)
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
