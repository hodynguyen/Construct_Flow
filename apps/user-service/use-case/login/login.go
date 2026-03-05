package login

import (
	"context"

	"golang.org/x/crypto/bcrypt"

	"github.com/hodynguyen/construct-flow/apps/user-service/common"
	"github.com/hodynguyen/construct-flow/apps/user-service/domain"
	"github.com/hodynguyen/construct-flow/apps/user-service/entity/dto"
)

// UseCase handles email+password authentication and JWT issuance.
type UseCase struct {
	userRepo     domain.UserRepository
	tokenService domain.TokenService
}

func New(userRepo domain.UserRepository, tokenService domain.TokenService) *UseCase {
	return &UseCase{userRepo: userRepo, tokenService: tokenService}
}

func (uc *UseCase) Execute(ctx context.Context, req dto.LoginRequest) (*dto.LoginResponse, error) {
	if req.Email == "" || req.Password == "" {
		return nil, common.ErrInvalidInput
	}

	user, err := uc.userRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		// Do not leak whether the email exists.
		return nil, common.ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, common.ErrInvalidCredentials
	}

	token, err := uc.tokenService.GenerateToken(domain.TokenClaims{
		UserID:    user.ID,
		CompanyID: user.CompanyID,
		Role:      user.Role,
	})
	if err != nil {
		return nil, err
	}

	return &dto.LoginResponse{
		AccessToken: token,
		User: dto.UserResponse{
			ID:        user.ID,
			CompanyID: user.CompanyID,
			Email:     user.Email,
			Name:      user.Name,
			Role:      user.Role,
			CreatedAt: user.CreatedAt,
		},
	}, nil
}
