package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/rtu-api/internal/db/sqlc"
	"github.com/rtu-api/internal/httpx"
	"github.com/rtu-api/internal/repository"
)

// CalibrationInstrumentService applies the business rules of the reference
// instruments used to calibrate devices.
type CalibrationInstrumentService struct {
	repo *repository.CalibrationInstrumentRepository
}

// CalibrationInstrumentCreateInput is the POST /calibration-instruments body.
type CalibrationInstrumentCreateInput struct {
	Name            string      `json:"name" validate:"required,max=100"`
	Manufacturer    *string     `json:"manufacturer" validate:"omitempty,max=100"`
	Model           *string     `json:"model" validate:"omitempty,max=100"`
	SerialNumber    *string     `json:"serial_number" validate:"omitempty,max=100"`
	CalibrationDate *httpx.Date `json:"calibration_date"`
	ExpireDate      *httpx.Date `json:"expire_date"`
	Active          *bool       `json:"active"`
}

// CalibrationInstrumentUpdateInput is the PATCH body.
type CalibrationInstrumentUpdateInput struct {
	Name            *string     `json:"name" validate:"omitempty,max=100"`
	Manufacturer    *string     `json:"manufacturer" validate:"omitempty,max=100"`
	Model           *string     `json:"model" validate:"omitempty,max=100"`
	SerialNumber    *string     `json:"serial_number" validate:"omitempty,max=100"`
	CalibrationDate *httpx.Date `json:"calibration_date"`
	ExpireDate      *httpx.Date `json:"expire_date"`
	Active          *bool       `json:"active"`
}

func validateCertificateDates(calibrationDate, expireDate *time.Time) error {
	if calibrationDate == nil || expireDate == nil {
		return nil
	}
	if !expireDate.After(*calibrationDate) {
		return httpx.Err(httpx.ErrInstrumentDates).
			WithField("expire_date", httpx.IssueInvalid, "Must be later than calibration_date.")
	}
	return nil
}

// List returns one page of instruments.
func (s *CalibrationInstrumentService) List(ctx context.Context, page httpx.Page, filter repository.CalibrationInstrumentFilter) ([]repository.CalibrationInstrumentListItem, int64, error) {
	return s.repo.List(ctx, page, filter)
}

// Get returns a single instrument.
func (s *CalibrationInstrumentService) Get(ctx context.Context, id uuid.UUID) (sqlc.CalibrationInstrument, error) {
	return s.repo.Get(ctx, id)
}

// Create registers a new instrument.
func (s *CalibrationInstrumentService) Create(ctx context.Context, in CalibrationInstrumentCreateInput) (sqlc.CalibrationInstrument, error) {
	if err := validateCertificateDates(in.CalibrationDate.AsTime(), in.ExpireDate.AsTime()); err != nil {
		return sqlc.CalibrationInstrument{}, err
	}

	return s.repo.Create(ctx, sqlc.CreateCalibrationInstrumentParams{
		Name:            in.Name,
		Manufacturer:    in.Manufacturer,
		Model:           in.Model,
		SerialNumber:    in.SerialNumber,
		CalibrationDate: in.CalibrationDate,
		ExpireDate:      in.ExpireDate,
		Active:          in.Active,
	})
}

// Update applies a partial update to an instrument.
func (s *CalibrationInstrumentService) Update(ctx context.Context, id uuid.UUID, fields httpx.FieldSet, in CalibrationInstrumentUpdateInput) (sqlc.CalibrationInstrument, error) {
	params := sqlc.UpdateCalibrationInstrumentParams{ID: id}

	name, setName, err := patchRequired(fields, "name", in.Name)
	if err != nil {
		return sqlc.CalibrationInstrument{}, err
	}
	params.Name, params.NameDoUpdate = name, setName

	active, setActive, err := patchRequired(fields, "active", in.Active)
	if err != nil {
		return sqlc.CalibrationInstrument{}, err
	}
	params.Active, params.ActiveDoUpdate = active, setActive

	params.Manufacturer, params.ManufacturerDoUpdate = patchNullable(fields, "manufacturer", in.Manufacturer)
	params.Model, params.ModelDoUpdate = patchNullable(fields, "model", in.Model)
	params.SerialNumber, params.SerialNumberDoUpdate = patchNullable(fields, "serial_number", in.SerialNumber)

	calibrationDate, setCalibration := patchNullable(fields, "calibration_date", in.CalibrationDate)
	params.CalibrationDate, params.CalibrationDateDoUpdate = calibrationDate, setCalibration

	expireDate, setExpire := patchNullable(fields, "expire_date", in.ExpireDate)
	params.ExpireDate, params.ExpireDateDoUpdate = expireDate, setExpire

	// Compare against the stored values for whichever date was not supplied.
	if setCalibration || setExpire {
		current, err := s.repo.Get(ctx, id)
		if err != nil {
			return sqlc.CalibrationInstrument{}, err
		}
		effectiveCalibration := current.CalibrationDate.AsTime()
		if setCalibration {
			effectiveCalibration = params.CalibrationDate.AsTime()
		}
		effectiveExpire := current.ExpireDate.AsTime()
		if setExpire {
			effectiveExpire = params.ExpireDate.AsTime()
		}
		if err := validateCertificateDates(effectiveCalibration, effectiveExpire); err != nil {
			return sqlc.CalibrationInstrument{}, err
		}
	}

	return s.repo.Update(ctx, params)
}

// SoftDelete deactivates an instrument.
func (s *CalibrationInstrumentService) SoftDelete(ctx context.Context, id uuid.UUID) (sqlc.CalibrationInstrument, error) {
	return s.repo.SetActive(ctx, id, false)
}

// Restore reactivates a soft-deleted instrument.
func (s *CalibrationInstrumentService) Restore(ctx context.Context, id uuid.UUID) (sqlc.CalibrationInstrument, error) {
	return s.repo.SetActive(ctx, id, true)
}

// Purge removes an instrument permanently. It fails while calibrations
// reference it.
func (s *CalibrationInstrumentService) Purge(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
