package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/rtu-api/internal/db/sqlc"
	"github.com/rtu-api/internal/httpx"
	"github.com/rtu-api/internal/repository"
)

// EngineerService applies the business rules of rtu.engineers.
type EngineerService struct {
	repo *repository.EngineerRepository
}

// EngineerCreateInput is the POST /engineers body.
type EngineerCreateInput struct {
	FullName  string  `json:"full_name" validate:"required,max=255"`
	LicenseNo *string `json:"license_no" validate:"omitempty,max=100"`
	Position  *string `json:"position" validate:"omitempty,max=255"`
	Active    *bool   `json:"active"`
}

// EngineerUpdateInput is the PATCH /engineers/{id} body.
type EngineerUpdateInput struct {
	FullName  *string `json:"full_name" validate:"omitempty,max=255"`
	LicenseNo *string `json:"license_no" validate:"omitempty,max=100"`
	Position  *string `json:"position" validate:"omitempty,max=255"`
	Active    *bool   `json:"active"`
}

// List returns one page of engineers.
func (s *EngineerService) List(ctx context.Context, page httpx.Page, filter repository.EngineerFilter) ([]repository.EngineerListItem, int64, error) {
	return s.repo.List(ctx, page, filter)
}

// Get returns a single engineer.
func (s *EngineerService) Get(ctx context.Context, id uuid.UUID) (sqlc.Engineer, error) {
	return s.repo.Get(ctx, id)
}

// Create registers a new engineer.
func (s *EngineerService) Create(ctx context.Context, in EngineerCreateInput) (sqlc.Engineer, error) {
	return s.repo.Create(ctx, sqlc.CreateEngineerParams{
		FullName:  in.FullName,
		LicenseNo: in.LicenseNo,
		Position:  in.Position,
		Active:    in.Active,
	})
}

// Update applies a partial update to an engineer.
func (s *EngineerService) Update(ctx context.Context, id uuid.UUID, fields httpx.FieldSet, in EngineerUpdateInput) (sqlc.Engineer, error) {
	params := sqlc.UpdateEngineerParams{ID: id}

	fullName, setFullName, err := patchRequired(fields, "full_name", in.FullName)
	if err != nil {
		return sqlc.Engineer{}, err
	}
	params.FullName, params.FullNameDoUpdate = fullName, setFullName

	active, setActive, err := patchRequired(fields, "active", in.Active)
	if err != nil {
		return sqlc.Engineer{}, err
	}
	params.Active, params.ActiveDoUpdate = active, setActive

	params.LicenseNo, params.LicenseNoDoUpdate = patchNullable(fields, "license_no", in.LicenseNo)
	params.Position, params.PositionDoUpdate = patchNullable(fields, "position", in.Position)

	return s.repo.Update(ctx, params)
}

// SoftDelete deactivates an engineer.
func (s *EngineerService) SoftDelete(ctx context.Context, id uuid.UUID) (sqlc.Engineer, error) {
	return s.repo.SetActive(ctx, id, false)
}

// Restore reactivates a deactivated engineer.
func (s *EngineerService) Restore(ctx context.Context, id uuid.UUID) (sqlc.Engineer, error) {
	return s.repo.SetActive(ctx, id, true)
}

// Purge removes an engineer permanently. It fails while PM reports reference it.
func (s *EngineerService) Purge(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
