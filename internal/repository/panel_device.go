package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rtu-api/internal/db"
	"github.com/rtu-api/internal/db/sqlc"
	"github.com/rtu-api/internal/domain"
	"github.com/rtu-api/internal/httpx"
)

// PanelDeviceRepository reads and writes rtu.panel_devices.
type PanelDeviceRepository struct {
	pool *pgxpool.Pool
	q    *sqlc.Queries
}

var panelDeviceConstraints = db.Constraints{
	"uk_device_serial":              httpx.ErrDeviceSerialDup,
	"uk_panel_device_tag":           httpx.ErrDeviceTagDup,
	"fk_panel_devices_panel":        httpx.ErrPanelNotFound,
	"fk_panel_devices_device_model": httpx.ErrDeviceModelNotFnd,
}

var panelDeviceDeleteConstraints = db.Constraints{
	"fk_calibrations_panel_device": httpx.ErrDeviceInUse,
}

var panelDeviceSortable = httpx.Sortable{
	"tag_name":             "pd.tag_name",
	"serial_number":        "pd.serial_number",
	"asset_code":           "pd.asset_code",
	"communication_status": "pd.communication_status",
	"health_status":        "pd.health_status",
	"installed_at":         "pd.installed_at",
	"last_seen_at":         "pd.last_seen_at",
	"active":               "pd.active",
	"panel_code":           "p.code",
	"device_model_code":    "dm.code",
	"created_at":           "pd.created_at",
	"updated_at":           "pd.updated_at",
}

// PanelDeviceSortable lists the sort keys accepted by the list endpoint.
func PanelDeviceSortable() httpx.Sortable { return panelDeviceSortable }

// PanelDeviceFilter narrows a panel device list query.
type PanelDeviceFilter struct {
	PanelID             *uuid.UUID
	DeviceModelID       *uuid.UUID
	Active              *bool
	CommunicationStatus *string
	HealthStatus        *string
	InstalledFrom       *time.Time
	InstalledTo         *time.Time
	LastSeenFrom        *time.Time
	LastSeenTo          *time.Time
	NeverSeen           *bool
}

// PanelDeviceView is a panel device joined with its panel, its model and the
// summary of its calibration history.
type PanelDeviceView struct {
	sqlc.PanelDevice
	// OperationalStatus is computed in Go from CommunicationStatus + HealthStatus
	// (see domain.DeviceOperationalStatus) — never stored, populated after the
	// row is scanned so the frontend never needs to know the mapping rule.
	OperationalStatus       string     `db:"-" json:"operational_status"`
	PanelCode               string     `db:"panel_code" json:"panel_code"`
	PanelLocation           *string    `db:"panel_location" json:"panel_location"`
	PanelActive             bool       `db:"panel_active" json:"panel_active"`
	DeviceModelCode         string     `db:"device_model_code" json:"device_model_code"`
	DeviceModelName         string     `db:"device_model_name" json:"device_model_name"`
	DeviceModelManufacturer *string    `db:"device_model_manufacturer" json:"device_model_manufacturer"`
	CalibrationCount        int64      `db:"calibration_count" json:"calibration_count"`
	LastCalibratedAt        *time.Time `db:"last_calibrated_at" json:"last_calibrated_at"`
	LastCalibrationResult   *string    `db:"last_calibration_result" json:"last_calibration_result"`
	TotalCount              int64      `db:"total_count" json:"-"`
}

// withOperationalStatus fills the computed field after the row scan.
func withOperationalStatus(v PanelDeviceView) PanelDeviceView {
	v.OperationalStatus = domain.DeviceOperationalStatus(v.CommunicationStatus, v.HealthStatus)
	return v
}

const panelDeviceColumns = `
    pd.id, pd.panel_id, pd.device_model_id, pd.tag_name, pd.serial_number, pd.asset_code,
    pd.firmware_version, pd.communication_status, pd.health_status, pd.installed_at,
    pd.last_seen_at, pd.note, pd.active, pd.created_at, pd.updated_at,
    pd.created_by, pd.updated_by,
    p.code AS panel_code,
    p.location AS panel_location,
    p.active AS panel_active,
    dm.code AS device_model_code,
    dm.name AS device_model_name,
    dm.manufacturer AS device_model_manufacturer,
    (SELECT count(*) FROM rtu.calibrations c WHERE c.panel_device_id = pd.id)::bigint AS calibration_count,
    last_cal.performed_at AS last_calibrated_at,
    last_cal.result AS last_calibration_result`

const panelDeviceFrom = `
FROM rtu.panel_devices pd
JOIN rtu.panels p ON p.id = pd.panel_id
JOIN rtu.device_models dm ON dm.id = pd.device_model_id
LEFT JOIN LATERAL (
    SELECT c.performed_at, c.result
    FROM rtu.calibrations c
    WHERE c.panel_device_id = pd.id
    ORDER BY c.performed_at DESC
    LIMIT 1
) last_cal ON TRUE`

// List returns one page of panel devices together with the total row count.
func (r *PanelDeviceRepository) List(ctx context.Context, page httpx.Page, filter PanelDeviceFilter) ([]PanelDeviceView, int64, error) {
	a := &args{}
	conds := conditions{}

	if filter.PanelID != nil {
		conds = append(conds, "pd.panel_id = "+a.add(*filter.PanelID))
	}
	if filter.DeviceModelID != nil {
		conds = append(conds, "pd.device_model_id = "+a.add(*filter.DeviceModelID))
	}
	if filter.Active != nil {
		conds = append(conds, "pd.active = "+a.add(*filter.Active))
	}
	if filter.CommunicationStatus != nil {
		conds = append(conds, "pd.communication_status = "+a.add(*filter.CommunicationStatus))
	}
	if filter.HealthStatus != nil {
		conds = append(conds, "pd.health_status = "+a.add(*filter.HealthStatus))
	}
	if filter.InstalledFrom != nil {
		conds = append(conds, "pd.installed_at >= "+a.add(*filter.InstalledFrom))
	}
	if filter.InstalledTo != nil {
		conds = append(conds, "pd.installed_at <= "+a.add(*filter.InstalledTo))
	}
	if filter.LastSeenFrom != nil {
		conds = append(conds, "pd.last_seen_at >= "+a.add(*filter.LastSeenFrom))
	}
	if filter.LastSeenTo != nil {
		conds = append(conds, "pd.last_seen_at <= "+a.add(*filter.LastSeenTo))
	}
	if filter.NeverSeen != nil {
		if *filter.NeverSeen {
			conds = append(conds, "pd.last_seen_at IS NULL")
		} else {
			conds = append(conds, "pd.last_seen_at IS NOT NULL")
		}
	}
	if page.Search != nil {
		p := a.add(likePattern(*page.Search))
		conds = append(conds, fmt.Sprintf(
			`(pd.tag_name ILIKE %s ESCAPE '\' OR pd.serial_number ILIKE %s ESCAPE '\' OR pd.asset_code ILIKE %s ESCAPE '\' OR p.code ILIKE %s ESCAPE '\')`,
			p, p, p, p,
		))
	}

	query := fmt.Sprintf(
		"SELECT %s,\n    count(*) OVER ()::bigint AS total_count%s\nWHERE %s\nORDER BY %s %s NULLS LAST, pd.id %s\nLIMIT %s OFFSET %s",
		panelDeviceColumns, panelDeviceFrom, conds.where(),
		page.SortSQL, page.Order, page.Order,
		a.add(page.RowLimit()), a.add(page.Offset()),
	)

	rows, err := r.pool.Query(ctx, query, a.values...)
	if err != nil {
		return nil, 0, db.Translate(err)
	}

	items, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[PanelDeviceView])
	if err != nil {
		return nil, 0, db.Translate(err)
	}
	for i := range items {
		items[i] = withOperationalStatus(items[i])
	}

	var total int64
	if len(items) > 0 {
		total = items[0].TotalCount
	}
	return items, total, nil
}

// GetView returns a single panel device with its joined context.
func (r *PanelDeviceRepository) GetView(ctx context.Context, id uuid.UUID) (PanelDeviceView, error) {
	query := fmt.Sprintf("SELECT %s%s\nWHERE pd.id = $1", panelDeviceColumns, panelDeviceFrom)

	rows, err := r.pool.Query(ctx, query, id)
	if err != nil {
		return PanelDeviceView{}, db.Translate(err)
	}

	view, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByNameLax[PanelDeviceView])
	if err != nil {
		return PanelDeviceView{}, db.Translate(err, db.WithNotFound(httpx.ErrPanelDeviceNotFnd))
	}
	return withOperationalStatus(view), nil
}

// Get returns the raw row of a panel device.
func (r *PanelDeviceRepository) Get(ctx context.Context, id uuid.UUID) (sqlc.PanelDevice, error) {
	device, err := r.q.GetPanelDevice(ctx, id)
	if err != nil {
		return sqlc.PanelDevice{}, db.Translate(err, db.WithNotFound(httpx.ErrPanelDeviceNotFnd))
	}
	return device, nil
}

// Create inserts a panel device.
func (r *PanelDeviceRepository) Create(ctx context.Context, arg sqlc.CreatePanelDeviceParams) (sqlc.PanelDevice, error) {
	arg.CreatedBy, arg.UpdatedBy = createAudit(ctx)
	device, err := r.q.CreatePanelDevice(ctx, arg)
	if err != nil {
		return sqlc.PanelDevice{}, db.Translate(err, db.Options{Constraints: panelDeviceConstraints})
	}
	return device, nil
}

// Update applies a partial update.
func (r *PanelDeviceRepository) Update(ctx context.Context, arg sqlc.UpdatePanelDeviceParams) (sqlc.PanelDevice, error) {
	arg.UpdatedBy = updateAudit(ctx)
	device, err := r.q.UpdatePanelDevice(ctx, arg)
	if err != nil {
		return sqlc.PanelDevice{}, db.Translate(err, db.Options{
			NotFound:    &httpx.ErrPanelDeviceNotFnd,
			Constraints: panelDeviceConstraints,
		})
	}
	return device, nil
}

// UpdateStatus records a telemetry heartbeat.
func (r *PanelDeviceRepository) UpdateStatus(ctx context.Context, arg sqlc.UpdatePanelDeviceStatusParams) (sqlc.PanelDevice, error) {
	arg.UpdatedBy = updateAudit(ctx)
	device, err := r.q.UpdatePanelDeviceStatus(ctx, arg)
	if err != nil {
		return sqlc.PanelDevice{}, db.Translate(err, db.Options{
			NotFound:    &httpx.ErrPanelDeviceNotFnd,
			Constraints: panelDeviceConstraints,
		})
	}
	return device, nil
}

// SetActive soft-deletes or restores a panel device.
func (r *PanelDeviceRepository) SetActive(ctx context.Context, id uuid.UUID, active bool) (sqlc.PanelDevice, error) {
	device, err := r.q.SetPanelDeviceActive(ctx, sqlc.SetPanelDeviceActiveParams{
		ID: id, Active: active, UpdatedBy: updateAudit(ctx),
	})
	if err != nil {
		return sqlc.PanelDevice{}, db.Translate(err, db.WithNotFound(httpx.ErrPanelDeviceNotFnd))
	}
	return device, nil
}

// Delete removes a panel device permanently.
func (r *PanelDeviceRepository) Delete(ctx context.Context, id uuid.UUID) error {
	affected, err := r.q.DeletePanelDevice(ctx, id)
	if err != nil {
		return db.Translate(err, db.Options{Constraints: panelDeviceDeleteConstraints})
	}
	if affected == 0 {
		return httpx.Err(httpx.ErrPanelDeviceNotFnd)
	}
	return nil
}

// IsActive reports whether a device exists and is active.
func (r *PanelDeviceRepository) IsActive(ctx context.Context, id uuid.UUID) (bool, error) {
	active, err := r.q.PanelDeviceIsActive(ctx, id)
	if err != nil {
		return false, db.Translate(err, db.WithNotFound(httpx.ErrPanelDeviceNotFnd))
	}
	return active, nil
}
