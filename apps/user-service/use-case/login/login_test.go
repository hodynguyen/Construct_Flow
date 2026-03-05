package login_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"golang.org/x/crypto/bcrypt"

	"github.com/hodynguyen/construct-flow/apps/user-service/common"
	"github.com/hodynguyen/construct-flow/apps/user-service/domain"
	"github.com/hodynguyen/construct-flow/apps/user-service/entity/dto"
	"github.com/hodynguyen/construct-flow/apps/user-service/entity/model"
	"github.com/hodynguyen/construct-flow/apps/user-service/mock"
	"github.com/hodynguyen/construct-flow/apps/user-service/use-case/login"
)

func TestLogin(t *testing.T) {
	const (
		companyID   = "company-1"
		userID      = "user-1"
		email       = "manager@acme.com"
		testCredential = "testcred_abc123"
		fakeToken   = "eyJhbGciOiJSUzI1NiJ9.test"
	)

	hash, _ := bcrypt.GenerateFromPassword([]byte(testCredential), 4) // cost=4 for speed in tests
	baseUser := &model.User{
		ID:        userID,
		CompanyID: companyID,
		Email:     email,
		Name:      "Suzuki Manager",
		Password:  string(hash),
		Role:      "manager",
	}

	tests := []struct {
		name    string
		req     dto.LoginRequest
		setup   func(ur *mock.MockUserRepository, ts *mock.MockTokenService)
		wantErr error
		check   func(t *testing.T, resp *dto.LoginResponse)
	}{
		{
			name: "success",
			req:  dto.LoginRequest{Email: email, Password: testCredential},
			setup: func(ur *mock.MockUserRepository, ts *mock.MockTokenService) {
				ur.EXPECT().FindByEmail(gomock.Any(), email).Return(baseUser, nil)
				ts.EXPECT().GenerateToken(domain.TokenClaims{
					UserID:    userID,
					CompanyID: companyID,
					Role:      "manager",
				}).Return(fakeToken, nil)
			},
			check: func(t *testing.T, resp *dto.LoginResponse) {
				assert.Equal(t, fakeToken, resp.AccessToken)
				assert.Equal(t, email, resp.User.Email)
			},
		},
		{
			name: "wrong_password",
			req:  dto.LoginRequest{Email: email, Password: "wrongcred"},
			setup: func(ur *mock.MockUserRepository, ts *mock.MockTokenService) {
				ur.EXPECT().FindByEmail(gomock.Any(), email).Return(baseUser, nil)
			},
			wantErr: common.ErrInvalidCredentials,
		},
		{
			name: "user_not_found",
			req:  dto.LoginRequest{Email: "nobody@acme.com", Password: testCredential},
			setup: func(ur *mock.MockUserRepository, ts *mock.MockTokenService) {
				ur.EXPECT().FindByEmail(gomock.Any(), "nobody@acme.com").Return(nil, common.ErrNotFound)
			},
			wantErr: common.ErrInvalidCredentials,
		},
		{
			name:    "invalid_input — missing email",
			req:     dto.LoginRequest{Password: testCredential},
			setup:   func(ur *mock.MockUserRepository, ts *mock.MockTokenService) {},
			wantErr: common.ErrInvalidInput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ur := mock.NewMockUserRepository(ctrl)
			ts := mock.NewMockTokenService(ctrl)
			tt.setup(ur, ts)

			uc := login.New(ur, ts)
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
