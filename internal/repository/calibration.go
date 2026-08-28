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
	"github.com/rtu-api/internal/httpx"
)

// CalibrationRepository reads and writes rtu.calibrations.
type CalibrationRepository struct {
	pool *pgxpool.Pool
	q    *sqlc.Queries
}

var calibrationConstraints = db.Constraints{
	"fk_calibrations_panel_device": httpx.ErrPanelDeviceNotFnd,
	"fk_calibrations_instrument":   httpx.ErrInstrumentNotFnd,
	"ck_calibrations_result":       httpx.ErrCalibrationResult,
}

var calibrationDeleteConstraints = db.Constraints{
	"fk_calibration_readings_calibration": httpx.ErrReferenced,
}

var calibrationSortable = httpx.Sortable{
	"performed_at": "c.performed_at",
	"performed_by": "c.performed_by",
	"result":       "c.result",
	"panel_code":   "p.code",
	"created_at":   "c.created_at",
	"updated_at":   "c.updated_at",
}

// CalibrationSortable lists the sort keys accepted by the list endpoint.
func CalibrationSortable() httpx.Sortable { return calibrationSortable }

// CalibrationFilter narrows a calibration list query.
type CalibrationFilter struct {
	PanelDeviceID *uuid.UUID
	PanelID       *uuid.UUID
	EquipmentType *string
	InstrumentID  *uuid.UUID
	Result        *string
	PerformedBy   *string
	PerformedFrom *time.Time
	PerformedTo   *time.Time
}

// CalibrationView is a calibration joined with the device, panel and instrument.
type CalibrationView struct {
	sqlc.Calibration
	DeviceTagName          *string    `db:"device_tag_name" json:"device_tag_name"`
	DeviceSerialNumber     *string    `db:"device_serial_number" json:"device_serial_number"`
	DeviceName             string     `db:"device_name" json:"device_name"`
	DeviceEquipmentType    *string    `db:"device_equipment_type" json:"device_equipment_type"`
	DeviceManufacturer     *string    `db:"device_manufacturer" json:"device_manufacturer"`
	DeviceBrand            *string    `db:"device_brand" json:"device_brand"`
	DeviceModel            *string    `db:"device_model" json:"device_model"`
	PanelID                uuid.UUID  `db:"panel_id" json:"panel_id"`
	PanelCode              string     `db:"panel_code" json:"panel_code"`
	InstrumentName         string     `db:"instrument_name" json:"instrument_name"`
	InstrumentSerialNumber *string    `db:"instrument_serial_number" json:"instrument_serial_number"`
	InstrumentExpireDate   *time.Time `db:"instrument_expire_date" json:"instrument_expire_date"`
	ReadingCount           int64      `db:"reading_count" json:"reading_count"`
	TotalCount             int64      `db:"total_count" json:"-"`
}

// CalibrationDetail is a calibration view with its readings attached.
type CalibrationDetail struct {
	CalibrationView
	Readings []sqlc.CalibrationReading `json:"readings"`
}

const calibrationColumns = `
    c.id, c.panel_device_id, c.instrument_id, c.performed_by, c.performed_at,
    c.result, c.remark, c.created_at, c.updated_at, c.created_by, c.updated_by,
    pd.tag_name AS device_tag_name,
    pd.serial_number AS device_serial_number,
    pd.name AS device_name,
    pd.equipment_type AS device_equipment_type,
    pd.manufacturer AS device_manufacturer,
    pd.brand AS device_brand,
    pd.model AS device_model,
    pd.panel_id AS panel_id,
    p.code AS panel_code,
    ci.name AS instrument_name,
    ci.serial_number AS instrument_serial_number,
    ci.expire_date AS instrument_expire_date,
    (SELECT count(*) FROM rtu.calibration_readings cr WHERE cr.calibration_id = c.id)::bigint AS reading_count`

const calibrationFrom = `
FROM rtu.calibrations c
JOIN rtu.panel_devices pd ON pd.id = c.panel_device_id
JOIN rtu.panels p ON p.id = pd.panel_id
JOIN rtu.calibration_instruments ci ON ci.id = c.instrument_id`

// List returns one page of calibrations together with the total row count.
func (r *CalibrationRepository) List(ctx context.Context, page httpx.Page, filter CalibrationFilter) ([]CalibrationView, int64, error) {
	a := &args{}
	conds := conditions{}

	if filter.PanelDeviceID != nil {
		conds = append(conds, "c.panel_device_id = "+a.add(*filter.PanelDeviceID))
	}
	if filter.PanelID != nil {
		conds = append(conds, "pd.panel_id = "+a.add(*filter.PanelID))
	}
	if filter.EquipmentType != nil {
		conds = append(conds, "pd.equipment_type = "+a.add(*filter.EquipmentType))
	}
	if filter.InstrumentID != nil {
		conds = append(conds, "c.instrument_id = "+a.add(*filter.InstrumentID))
	}
	if filter.Result != nil {
		conds = append(conds, "c.result = "+a.add(*filter.Result))
	}
	if filter.PerformedBy != nil {
		conds = append(conds, "c.performed_by = "+a.add(*filter.PerformedBy))
	}
	if filter.PerformedFrom != nil {
		conds = append(conds, "c.performed_at >= "+a.add(*filter.PerformedFrom))
	}
	if filter.PerformedTo != nil {
		conds = append(conds, "c.performed_at <= "+a.add(*filter.PerformedTo))
	}
	if page.Search != nil {
		p := a.add(likePattern(*page.Search))
		conds = append(conds, fmt.Sprintf(
			`(c.performed_by ILIKE %s ESCAPE '\' OR c.remark ILIKE %s ESCAPE '\' OR pd.name ILIKE %s ESCAPE '\' OR pd.tag_name ILIKE %s ESCAPE '\' OR pd.serial_number ILIKE %s ESCAPE '\' OR p.code ILIKE %s ESCAPE '\')`,
			p, p, p, p, p, p,
		))
	}

	query := fmt.Sprintf(
		"SELECT %s,\n    count(*) OVER ()::bigint AS total_count%s\nWHERE %s\nORDER BY %s %s NULLS LAST, c.id %s\nLIMIT %s OFFSET %s",
		calibrationColumns, calibrationFrom, conds.where(),
		page.SortSQL, page.Order, page.Order,
		a.add(page.RowLimit()), a.add(page.Offset()),
	)

	rows, err := r.pool.Query(ctx, query, a.values...)
	if err != nil {
		return nil, 0, db.Translate(err)
	}

	items, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[CalibrationView])
	if err != nil {
		return nil, 0, db.Translate(err)
	}

	var total int64
	if len(items) > 0 {
		total = items[0].TotalCount
	}
	return items, total, nil
}

// GetView returns a single calibration with its joined context.
func (r *CalibrationRepository) GetView(ctx context.Context, id uuid.UUID) (CalibrationView, error) {
	query := fmt.Sprintf("SELECT %s%s\nWHERE c.id = $1", calibrationColumns, calibrationFrom)

	rows, err := r.pool.Query(ctx, query, id)
	if err != nil {
		return CalibrationView{}, db.Translate(err)
	}

	view, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByNameLax[CalibrationView])
	if err != nil {
		return CalibrationView{}, db.Translate(err, db.WithNotFound(httpx.ErrCalibrationNotFnd))
	}
	return view, nil
}

// Get returns the raw row of a calibration.
func (r *CalibrationRepository) Get(ctx context.Context, id uuid.UUID) (sqlc.Calibration, error) {
	calibration, err := r.q.GetCalibration(ctx, id)
	if err != nil {
		return sqlc.Calibration{}, db.Translate(err, db.WithNotFound(httpx.ErrCalibrationNotFnd))
	}
	return calibration, nil
}

// Create inserts a calibration on its own.
func (r *CalibrationRepository) Create(ctx context.Context, arg sqlc.CreateCalibrationParams) (sqlc.Calibration, error) {
	arg.CreatedBy, arg.UpdatedBy = createAudit(ctx)
	calibration, err := r.q.CreateCalibration(ctx, arg)
	if err != nil {
		return sqlc.Calibration{}, db.Translate(err, db.Options{Constraints: calibrationConstraints})
	}
	return calibration, nil
}

// CreateWithReadings writes a calibration and its readings atomically, using
// COPY for the readings so a long measurement sheet stays a single round trip.
func (r *CalibrationRepository) CreateWithReadings(
	ctx context.Context,
	arg sqlc.CreateCalibrationParams,
	readings []sqlc.BulkCreateCalibrationReadingsParams,
) (sqlc.Calibration, []sqlc.CalibrationReading, error) {
	arg.CreatedBy, arg.UpdatedBy = createAudit(ctx)
	var (
		calibration sqlc.Calibration
		saved       []sqlc.CalibrationReading
	)

	err := db.InTx(ctx, r.pool, func(q *sqlc.Queries) error {
		created, err := q.CreateCalibration(ctx, arg)
		if err != nil {
			return db.Translate(err, db.Options{Constraints: calibrationConstraints})
		}
		calibration = created

		if len(readings) == 0 {
			saved = []sqlc.CalibrationReading{}
			return nil
		}

		stampBulkReadings(ctx, readings)
		for i := range readings {
			readings[i].CalibrationID = created.ID
		}
		if _, err := q.BulkCreateCalibrationReadings(ctx, readings); err != nil {
			return db.Translate(err, db.Options{Constraints: readingConstraints})
		}

		saved, err = q.ListCalibrationReadings(ctx, created.ID)
		if err != nil {
			return db.Translate(err)
		}
		return nil
	})
	if err != nil {
		return sqlc.Calibration{}, nil, db.Translate(err)
	}

	return calibration, saved, nil
}

// ReplaceReadings swaps the whole reading sheet of a calibration atomically.
func (r *CalibrationRepository) ReplaceReadings(
	ctx context.Context,
	calibrationID uuid.UUID,
	readings []sqlc.BulkCreateCalibrationReadingsParams,
) ([]sqlc.CalibrationReading, error) {
	var saved []sqlc.CalibrationReading

	err := db.InTx(ctx, r.pool, func(q *sqlc.Queries) error {
		found, err := q.CalibrationExists(ctx, calibrationID)
		if err != nil {
			return db.Translate(err)
		}
		if !found {
			return httpx.Err(httpx.ErrCalibrationNotFnd)
		}

		if _, err := q.DeleteCalibrationReadingsByCalibration(ctx, calibrationID); err != nil {
			return db.Translate(err)
		}

		if len(readings) > 0 {
			stampBulkReadings(ctx, readings)
			for i := range readings {
				readings[i].CalibrationID = calibrationID
			}
			if _, err := q.BulkCreateCalibrationReadings(ctx, readings); err != nil {
				return db.Translate(err, db.Options{Constraints: readingConstraints})
			}
		}

		saved, err = q.ListCalibrationReadings(ctx, calibrationID)
		if err != nil {
			return db.Translate(err)
		}
		return nil
	})
	if err != nil {
		return nil, db.Translate(err)
	}

	return saved, nil
}

// Update applies a partial update.
func (r *CalibrationRepository) Update(ctx context.Context, arg sqlc.UpdateCalibrationParams) (sqlc.Calibration, error) {
	arg.UpdatedBy = updateAudit(ctx)
	calibration, err := r.q.UpdateCalibration(ctx, arg)
	if err != nil {
		return sqlc.Calibration{}, db.Translate(err, db.Options{
			NotFound:    &httpx.ErrCalibrationNotFnd,
			Constraints: calibrationConstraints,
		})
	}
	return calibration, nil
}

// Delete removes a calibration; its readings cascade.
func (r *CalibrationRepository) Delete(ctx context.Context, id uuid.UUID) error {
	affected, err := r.q.DeleteCalibration(ctx, id)
	if err != nil {
		return db.Translate(err, db.Options{Constraints: calibrationDeleteConstraints})
	}
	if affected == 0 {
		return httpx.Err(httpx.ErrCalibrationNotFnd)
	}
	return nil
}

// ResultSummary counts calibrations per result for an optional device and
// period.
func (r *CalibrationRepository) ResultSummary(ctx context.Context, arg sqlc.CountCalibrationsByResultParams) ([]sqlc.CountCalibrationsByResultRow, error) {
	rows, err := r.q.CountCalibrationsByResult(ctx, arg)
	if err != nil {
		return nil, db.Translate(err)
	}
	return rows, nil
}

// ListByPmReport returns every calibration linked to a PM report (6-month PM).
func (r *CalibrationRepository) ListByPmReport(ctx context.Context, pmReportID uuid.UUID) ([]sqlc.Calibration, error) {
	rows, err := r.q.ListCalibrationsByPmReport(ctx, pmReportID)
	if err != nil {
		return nil, db.Translate(err)
	}
	return rows, nil
}
