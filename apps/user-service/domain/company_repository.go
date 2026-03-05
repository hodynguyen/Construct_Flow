package domain

import (
	"context"

	"github.com/hodynguyen/construct-flow/apps/user-service/entity/model"
)

type CompanyRepository interface {
	Create(ctx context.Context, company *model.Company) error
	FindByID(ctx context.Context, id string) (*model.Company, error)
}
