package register

import (
	"context"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"github.com/hodynguyen/construct-flow/apps/user-service/common"
	"github.com/hodynguyen/construct-flow/apps/user-service/domain"
	"github.com/hodynguyen/construct-flow/apps/user-service/entity/dto"
	"github.com/hodynguyen/construct-flow/apps/user-service/entity/model"
)

const bcryptCost = 12

// UseCase handles user registration, including optional company creation.
type UseCase struct {
	userRepo    domain.UserRepository
	companyRepo domain.CompanyRepository
}

func New(userRepo domain.UserRepository, companyRepo domain.CompanyRepository) *UseCase {
	return &UseCase{userRepo: userRepo, companyRepo: companyRepo}
}

func (uc *UseCase) Execute(ctx context.Context, req dto.RegisterRequest) (*dto.RegisterResponse, error) {
	if req.Email == "" || req.Password == "" || req.Name == "" {
		return nil, common.ErrInvalidInput
	}
	if req.Role == "" {
		req.Role = "worker"
	}

	var companyID string

	if req.CompanyID != "" {
		// Join existing company — verify it exists.
		company, err := uc.companyRepo.FindByID(ctx, req.CompanyID)
		if err != nil {
			return nil, err
		}
		companyID = company.ID
	} else if req.CompanyName != "" {
		// Create a new company.
		company := &model.Company{Name: req.CompanyName}
		if err := uc.companyRepo.Create(ctx, company); err != nil {
			return nil, fmt.Errorf("creating company: %w", err)
		}
		companyID = company.ID
	} else {
		return nil, fmt.Errorf("%w: company_id or company_name is required", common.ErrInvalidInput)
	}

	// Check email uniqueness within the company.
	exists, err := uc.userRepo.ExistsByEmail(ctx, req.Email, companyID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, common.ErrEmailExists
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("hashing password: %w", err)
	}

	user := &model.User{
		CompanyID: companyID,
		Email:     req.Email,
		Name:      req.Name,
		Password:  string(hash),
		Role:      req.Role,
	}
	if err := uc.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	return &dto.RegisterResponse{
		User: dto.UserResponse{
			ID:        user.ID,
			CompanyID: user.CompanyID,
			Email:     user.Email,
			Name:      user.Name,
			Role:      user.Role,
			CreatedAt: user.CreatedAt,
		},
		CompanyID: companyID,
	}, nil
}
