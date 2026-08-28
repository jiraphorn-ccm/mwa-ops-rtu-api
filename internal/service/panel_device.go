package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/rtu-api/internal/db/sqlc"
	"github.com/rtu-api/internal/httpx"
	"github.com/rtu-api/internal/repository"
)

// PanelDeviceService applies the business rules of rtu.panel_devices.
type PanelDeviceService struct {
	repo   *repository.PanelDeviceRepository
	panels *repository.PanelRepository
}

// PanelDeviceCreateInput is the POST /panel-devices body. Equipment fields are
// stored on the panel device row; device_models is optional UI master data only.
type PanelDeviceCreateInput struct {
	PanelID             uuid.UUID   `json:"panel_id"`
	Name                string      `json:"name" validate:"required,max=100"`
	EquipmentType       *string     `json:"equipment_type" validate:"omitempty,max=50"`
	Manufacturer        *string     `json:"manufacturer" validate:"omitempty,max=100"`
	Brand               *string     `json:"brand" validate:"omitempty,max=100"`
	Model               *string     `json:"model" validate:"omitempty,max=100"`
	SerialNumber        *string     `json:"serial_number" validate:"omitempty,max=100"`
	CalibrationDate     *httpx.Date `json:"calibration_date"`
	ExpireDate          *httpx.Date `json:"expire_date"`
	TagName             *string     `json:"tag_name" validate:"omitempty,max=100"`
	AssetCode           *string     `json:"asset_code" validate:"omitempty,max=100"`
	FirmwareVersion     *string     `json:"firmware_version" validate:"omitempty,max=50"`
	CommunicationStatus *string     `json:"communication_status" validate:"omitempty,oneof=ONLINE OFFLINE DEGRADED UNKNOWN"`
	HealthStatus        *string     `json:"health_status" validate:"omitempty,oneof=NORMAL WARNING CRITICAL UNKNOWN"`
	InstalledAt         *httpx.Date `json:"installed_at"`
	LastSeenAt          *time.Time  `json:"last_seen_at"`
	Note                *string     `json:"note" validate:"omitempty,max=4000"`
	Active              *bool       `json:"active"`
}

// PanelDeviceUpdateInput is the PATCH /panel-devices/{id} body.
type PanelDeviceUpdateInput struct {
	PanelID             *uuid.UUID  `json:"panel_id"`
	Name                *string     `json:"name" validate:"omitempty,max=100"`
	EquipmentType       *string     `json:"equipment_type" validate:"omitempty,max=50"`
	Manufacturer        *string     `json:"manufacturer" validate:"omitempty,max=100"`
	Brand               *string     `json:"brand" validate:"omitempty,max=100"`
	Model               *string     `json:"model" validate:"omitempty,max=100"`
	SerialNumber        *string     `json:"serial_number" validate:"omitempty,max=100"`
	CalibrationDate     *httpx.Date `json:"calibration_date"`
	ExpireDate          *httpx.Date `json:"expire_date"`
	TagName             *string     `json:"tag_name" validate:"omitempty,max=100"`
	AssetCode           *string     `json:"asset_code" validate:"omitempty,max=100"`
	FirmwareVersion     *string     `json:"firmware_version" validate:"omitempty,max=50"`
	CommunicationStatus *string     `json:"communication_status" validate:"omitempty,oneof=ONLINE OFFLINE DEGRADED UNKNOWN"`
	HealthStatus        *string     `json:"health_status" validate:"omitempty,oneof=NORMAL WARNING CRITICAL UNKNOWN"`
	InstalledAt         *httpx.Date `json:"installed_at"`
	LastSeenAt          *time.Time  `json:"last_seen_at"`
	Note                *string     `json:"note" validate:"omitempty,max=4000"`
	Active              *bool       `json:"active"`
}

// PanelDeviceStatusInput is the POST /panel-devices/{id}/status body used by
// the telemetry collector.
type PanelDeviceStatusInput struct {
	CommunicationStatus *string    `json:"communication_status" validate:"omitempty,oneof=ONLINE OFFLINE DEGRADED UNKNOWN"`
	HealthStatus        *string    `json:"health_status" validate:"omitempty,oneof=NORMAL WARNING CRITICAL UNKNOWN"`
	LastSeenAt          *time.Time `json:"last_seen_at"`
}

// List returns one page of panel devices.
func (s *PanelDeviceService) List(ctx context.Context, page httpx.Page, filter repository.PanelDeviceFilter) ([]repository.PanelDeviceView, int64, error) {
	return s.repo.List(ctx, page, filter)
}

// Get returns a single panel device with its joined context.
func (s *PanelDeviceService) Get(ctx context.Context, id uuid.UUID) (repository.PanelDeviceView, error) {
	return s.repo.GetView(ctx, id)
}

// Create attaches a device to a panel.
func (s *PanelDeviceService) Create(ctx context.Context, in PanelDeviceCreateInput) (repository.PanelDeviceView, error) {
	device, err := s.repo.Create(ctx, sqlc.CreatePanelDeviceParams{
		PanelID:             in.PanelID,
		Name:                in.Name,
		EquipmentType:       in.EquipmentType,
		Manufacturer:        in.Manufacturer,
		Brand:               in.Brand,
		Model:               in.Model,
		SerialNumber:        in.SerialNumber,
		CalibrationDate:     in.CalibrationDate,
		ExpireDate:          in.ExpireDate,
		TagName:             in.TagName,
		AssetCode:           in.AssetCode,
		FirmwareVersion:     in.FirmwareVersion,
		CommunicationStatus: in.CommunicationStatus,
		HealthStatus:        in.HealthStatus,
		InstalledAt:         in.InstalledAt,
		LastSeenAt:          in.LastSeenAt,
		Note:                in.Note,
		Active:              in.Active,
	})
	if err != nil {
		return repository.PanelDeviceView{}, err
	}
	return s.repo.GetView(ctx, device.ID)
}

// Update applies a partial update to a panel device.
func (s *PanelDeviceService) Update(ctx context.Context, id uuid.UUID, fields httpx.FieldSet, in PanelDeviceUpdateInput) (repository.PanelDeviceView, error) {
	params := sqlc.UpdatePanelDeviceParams{ID: id}

	panelID, setPanel, err := patchRequired(fields, "panel_id", in.PanelID)
	if err != nil {
		return repository.PanelDeviceView{}, err
	}
	params.PanelID, params.PanelIDDoUpdate = panelID, setPanel

	name, setName, err := patchRequired(fields, "name", in.Name)
	if err != nil {
		return repository.PanelDeviceView{}, err
	}
	params.Name, params.NameDoUpdate = name, setName

	comm, setComm, err := patchRequired(fields, "communication_status", in.CommunicationStatus)
	if err != nil {
		return repository.PanelDeviceView{}, err
	}
	params.CommunicationStatus, params.CommunicationStatusDoUpdate = comm, setComm

	health, setHealth, err := patchRequired(fields, "health_status", in.HealthStatus)
	if err != nil {
		return repository.PanelDeviceView{}, err
	}
	params.HealthStatus, params.HealthStatusDoUpdate = health, setHealth

	active, setActive, err := patchRequired(fields, "active", in.Active)
	if err != nil {
		return repository.PanelDeviceView{}, err
	}
	params.Active, params.ActiveDoUpdate = active, setActive

	params.EquipmentType, params.EquipmentTypeDoUpdate = patchNullable(fields, "equipment_type", in.EquipmentType)
	params.Manufacturer, params.ManufacturerDoUpdate = patchNullable(fields, "manufacturer", in.Manufacturer)
	params.Brand, params.BrandDoUpdate = patchNullable(fields, "brand", in.Brand)
	params.Model, params.ModelDoUpdate = patchNullable(fields, "model", in.Model)
	params.SerialNumber, params.SerialNumberDoUpdate = patchNullable(fields, "serial_number", in.SerialNumber)
	params.TagName, params.TagNameDoUpdate = patchNullable(fields, "tag_name", in.TagName)
	params.AssetCode, params.AssetCodeDoUpdate = patchNullable(fields, "asset_code", in.AssetCode)
	params.FirmwareVersion, params.FirmwareVersionDoUpdate = patchNullable(fields, "firmware_version", in.FirmwareVersion)
	params.Note, params.NoteDoUpdate = patchNullable(fields, "note", in.Note)
	params.LastSeenAt, params.LastSeenAtDoUpdate = patchNullable(fields, "last_seen_at", in.LastSeenAt)

	calibrationDate, setCalibration := patchNullable(fields, "calibration_date", in.CalibrationDate)
	params.CalibrationDate, params.CalibrationDateDoUpdate = calibrationDate, setCalibration

	expireDate, setExpire := patchNullable(fields, "expire_date", in.ExpireDate)
	params.ExpireDate, params.ExpireDateDoUpdate = expireDate, setExpire

	installedAt, setInstalled := patchNullable(fields, "installed_at", in.InstalledAt)
	params.InstalledAt, params.InstalledAtDoUpdate = installedAt, setInstalled

	if _, err := s.repo.Update(ctx, params); err != nil {
		return repository.PanelDeviceView{}, err
	}
	return s.repo.GetView(ctx, id)
}

// RecordStatus stores a telemetry heartbeat. last_seen_at defaults to now.
func (s *PanelDeviceService) RecordStatus(ctx context.Context, id uuid.UUID, in PanelDeviceStatusInput) (repository.PanelDeviceView, error) {
	params := sqlc.UpdatePanelDeviceStatusParams{ID: id, LastSeenAt: in.LastSeenAt}

	if in.CommunicationStatus != nil {
		params.CommunicationStatus = *in.CommunicationStatus
		params.CommunicationStatusDoUpdate = true
	}
	if in.HealthStatus != nil {
		params.HealthStatus = *in.HealthStatus
		params.HealthStatusDoUpdate = true
	}

	if _, err := s.repo.UpdateStatus(ctx, params); err != nil {
		return repository.PanelDeviceView{}, err
	}
	return s.repo.GetView(ctx, id)
}

// SoftDelete deactivates a panel device.
func (s *PanelDeviceService) SoftDelete(ctx context.Context, id uuid.UUID) (sqlc.PanelDevice, error) {
	return s.repo.SetActive(ctx, id, false)
}

// Restore reactivates a soft-deleted panel device.
func (s *PanelDeviceService) Restore(ctx context.Context, id uuid.UUID) (sqlc.PanelDevice, error) {
	return s.repo.SetActive(ctx, id, true)
}

// Purge removes a panel device permanently. It fails while calibrations exist.
func (s *PanelDeviceService) Purge(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
