package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/rtu-api/internal/db/sqlc"
	"github.com/rtu-api/internal/httpx"
	"github.com/rtu-api/internal/repository"
)

// PanelService applies the business rules of rtu.panels.
type PanelService struct {
	repo *repository.PanelRepository
}

// PanelCreateInput is the POST /panels body.
type PanelCreateInput struct {
	Code        string           `json:"code" validate:"required,max=20"`
	Location    *string          `json:"location" validate:"omitempty,max=4000"`
	Latitude    *decimal.Decimal `json:"latitude"`
	Longitude   *decimal.Decimal `json:"longitude"`
	InstallDate *httpx.Date      `json:"install_date"`
	Active      *bool            `json:"active"`
}

// PanelUpdateInput is the PATCH /panels/{id} body. Every field is optional and
// an explicit null clears a nullable column.
type PanelUpdateInput struct {
	Code        *string          `json:"code" validate:"omitempty,max=20"`
	Location    *string          `json:"location" validate:"omitempty,max=4000"`
	Latitude    *decimal.Decimal `json:"latitude"`
	Longitude   *decimal.Decimal `json:"longitude"`
	InstallDate *httpx.Date      `json:"install_date"`
	Active      *bool            `json:"active"`
}

func validateCoordinates(latitude, longitude *decimal.Decimal) error {
	var appErr *httpx.AppError
	appErr = checkDecimalRange(appErr, "latitude", latitude, -90, 90)
	appErr = checkDecimalRange(appErr, "longitude", longitude, -180, 180)
	return errOrNil(appErr)
}

// List returns one page of panels.
func (s *PanelService) List(ctx context.Context, page httpx.Page, filter repository.PanelFilter) ([]repository.PanelListItem, int64, error) {
	return s.repo.List(ctx, page, filter)
}

// Get returns a single panel with computed status.
func (s *PanelService) Get(ctx context.Context, id uuid.UUID) (repository.PanelDetail, error) {
	return s.repo.Get(ctx, id)
}

// GetByCode returns a single panel by its business code with computed status.
func (s *PanelService) GetByCode(ctx context.Context, code string) (repository.PanelDetail, error) {
	return s.repo.GetByCode(ctx, code)
}

// Create registers a new panel.
func (s *PanelService) Create(ctx context.Context, in PanelCreateInput) (repository.PanelDetail, error) {
	if err := validateCoordinates(in.Latitude, in.Longitude); err != nil {
		return repository.PanelDetail{}, err
	}

	return s.repo.Create(ctx, sqlc.CreatePanelParams{
		Code:        in.Code,
		Location:    in.Location,
		Latitude:    in.Latitude,
		Longitude:   in.Longitude,
		InstallDate: in.InstallDate,
		Active:      in.Active,
	})
}

// Update applies a partial update to a panel.
func (s *PanelService) Update(ctx context.Context, id uuid.UUID, fields httpx.FieldSet, in PanelUpdateInput) (repository.PanelDetail, error) {
	if err := validateCoordinates(in.Latitude, in.Longitude); err != nil {
		return repository.PanelDetail{}, err
	}

	params := sqlc.UpdatePanelParams{ID: id}

	code, setCode, err := patchRequired(fields, "code", in.Code)
	if err != nil {
		return repository.PanelDetail{}, err
	}
	params.Code, params.CodeDoUpdate = code, setCode

	active, setActive, err := patchRequired(fields, "active", in.Active)
	if err != nil {
		return repository.PanelDetail{}, err
	}
	params.Active, params.ActiveDoUpdate = active, setActive

	params.Location, params.LocationDoUpdate = patchNullable(fields, "location", in.Location)
	params.Latitude, params.LatitudeDoUpdate = patchNullable(fields, "latitude", in.Latitude)
	params.Longitude, params.LongitudeDoUpdate = patchNullable(fields, "longitude", in.Longitude)
	params.InstallDate, params.InstallDateDoUpdate = patchNullable(fields, "install_date", in.InstallDate)

	return s.repo.Update(ctx, params)
}

// SoftDelete deactivates a panel without losing its history.
func (s *PanelService) SoftDelete(ctx context.Context, id uuid.UUID) (repository.PanelDetail, error) {
	return s.repo.SetActive(ctx, id, false)
}

// Restore reactivates a soft-deleted panel.
func (s *PanelService) Restore(ctx context.Context, id uuid.UUID) (repository.PanelDetail, error) {
	return s.repo.SetActive(ctx, id, true)
}

// Purge removes a panel permanently. It fails while devices are attached.
func (s *PanelService) Purge(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
