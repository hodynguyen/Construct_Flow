package sql

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/hodynguyen/construct-flow/apps/user-service/common"
	"github.com/hodynguyen/construct-flow/apps/user-service/entity/model"
)

type companyRepository struct {
	db *gorm.DB
}

// NewCompanyRepository returns a GORM-backed CompanyRepository.
func NewCompanyRepository(db *gorm.DB) *companyRepository {
	return &companyRepository{db: db}
}

func (r *companyRepository) Create(ctx context.Context, company *model.Company) error {
	return r.db.WithContext(ctx).Create(company).Error
}

func (r *companyRepository) FindByID(ctx context.Context, companyID string) (*model.Company, error) {
	var company model.Company
	err := r.db.WithContext(ctx).Where("id = ?", companyID).First(&company).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, common.ErrCompanyNotFound
		}
		return nil, err
	}
	return &company, nil
}
