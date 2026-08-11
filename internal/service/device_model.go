package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/rtu-api/internal/db/sqlc"
	"github.com/rtu-api/internal/httpx"
	"github.com/rtu-api/internal/repository"
)

// DeviceModelService applies the business rules of rtu.device_models.
type DeviceModelService struct {
	repo *repository.DeviceModelRepository
}

// DeviceModelCreateInput is the POST /device-models body.
type DeviceModelCreateInput struct {
	Code         string  `json:"code" validate:"required,max=30"`
	Name         string  `json:"name" validate:"required,max=100"`
	Manufacturer *string `json:"manufacturer" validate:"omitempty,max=100"`
	Model        *string `json:"model" validate:"omitempty,max=100"`
	Description  *string `json:"description" validate:"omitempty,max=4000"`
	Active       *bool   `json:"active"`
}

// DeviceModelUpdateInput is the PATCH /device-models/{id} body.
type DeviceModelUpdateInput struct {
	Code         *string `json:"code" validate:"omitempty,max=30"`
	Name         *string `json:"name" validate:"omitempty,max=100"`
	Manufacturer *string `json:"manufacturer" validate:"omitempty,max=100"`
	Model        *string `json:"model" validate:"omitempty,max=100"`
	Description  *string `json:"description" validate:"omitempty,max=4000"`
	Active       *bool   `json:"active"`
}

// List returns one page of device models.
func (s *DeviceModelService) List(ctx context.Context, page httpx.Page, filter repository.DeviceModelFilter) ([]repository.DeviceModelListItem, int64, error) {
	return s.repo.List(ctx, page, filter)
}

// Get returns a single device model.
func (s *DeviceModelService) Get(ctx context.Context, id uuid.UUID) (sqlc.DeviceModel, error) {
	return s.repo.Get(ctx, id)
}

// GetByCode returns a single device model by its business code.
func (s *DeviceModelService) GetByCode(ctx context.Context, code string) (sqlc.DeviceModel, error) {
	return s.repo.GetByCode(ctx, code)
}

// Create registers a new device model.
func (s *DeviceModelService) Create(ctx context.Context, in DeviceModelCreateInput) (sqlc.DeviceModel, error) {
	return s.repo.Create(ctx, sqlc.CreateDeviceModelParams{
		Code:         in.Code,
		Name:         in.Name,
		Manufacturer: in.Manufacturer,
		Model:        in.Model,
		Description:  in.Description,
		Active:       in.Active,
	})
}

// Update applies a partial update to a device model.
func (s *DeviceModelService) Update(ctx context.Context, id uuid.UUID, fields httpx.FieldSet, in DeviceModelUpdateInput) (sqlc.DeviceModel, error) {
	params := sqlc.UpdateDeviceModelParams{ID: id}

	code, setCode, err := patchRequired(fields, "code", in.Code)
	if err != nil {
		return sqlc.DeviceModel{}, err
	}
	params.Code, params.CodeDoUpdate = code, setCode

	name, setName, err := patchRequired(fields, "name", in.Name)
	if err != nil {
		return sqlc.DeviceModel{}, err
	}
	params.Name, params.NameDoUpdate = name, setName

	active, setActive, err := patchRequired(fields, "active", in.Active)
	if err != nil {
		return sqlc.DeviceModel{}, err
	}
	params.Active, params.ActiveDoUpdate = active, setActive

	params.Manufacturer, params.ManufacturerDoUpdate = patchNullable(fields, "manufacturer", in.Manufacturer)
	params.Model, params.ModelDoUpdate = patchNullable(fields, "model", in.Model)
	params.Description, params.DescriptionDoUpdate = patchNullable(fields, "description", in.Description)

	return s.repo.Update(ctx, params)
}

// SoftDelete deactivates a device model.
func (s *DeviceModelService) SoftDelete(ctx context.Context, id uuid.UUID) (sqlc.DeviceModel, error) {
	return s.repo.SetActive(ctx, id, false)
}

// Restore reactivates a soft-deleted device model.
func (s *DeviceModelService) Restore(ctx context.Context, id uuid.UUID) (sqlc.DeviceModel, error) {
	return s.repo.SetActive(ctx, id, true)
}

// Purge removes a device model permanently. It fails while devices use it.
func (s *DeviceModelService) Purge(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
