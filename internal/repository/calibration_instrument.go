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

// CalibrationInstrumentRepository reads and writes rtu.calibration_instruments.
type CalibrationInstrumentRepository struct {
	pool *pgxpool.Pool
	q    *sqlc.Queries
}

var instrumentConstraints = db.Constraints{
	"uk_instrument_serial":                   httpx.ErrInstrumentSerialDup,
	"ck_instrument_expire_after_calibration": httpx.ErrInstrumentDates,
}

var instrumentDeleteConstraints = db.Constraints{
	"fk_calibrations_instrument": httpx.ErrInstrumentInUse,
}

var instrumentSortable = httpx.Sortable{
	"name":             "ci.name",
	"manufacturer":     "ci.manufacturer",
	"model":            "ci.model",
	"serial_number":    "ci.serial_number",
	"calibration_date": "ci.calibration_date",
	"expire_date":      "ci.expire_date",
	"active":           "ci.active",
	"created_at":       "ci.created_at",
	"updated_at":       "ci.updated_at",
}

// CalibrationInstrumentSortable lists the sort keys accepted by the endpoint.
func CalibrationInstrumentSortable() httpx.Sortable { return instrumentSortable }

// CalibrationInstrumentFilter narrows an instrument list query.
type CalibrationInstrumentFilter struct {
	Active       *bool
	Manufacturer *string
	// Expired selects instruments whose certificate is (or is not) past due.
	Expired *bool
	// ExpiringBefore selects certificates due before the given date.
	ExpiringBefore *time.Time
}

// CalibrationInstrumentListItem adds derived certificate state to the row.
type CalibrationInstrumentListItem struct {
	sqlc.CalibrationInstrument
	IsExpired        bool  `db:"is_expired" json:"is_expired"`
	DaysUntilExpiry  *int  `db:"days_until_expiry" json:"days_until_expiry"`
	CalibrationCount int64 `db:"calibration_count" json:"calibration_count"`
	TotalCount       int64 `db:"total_count" json:"-"`
}

const instrumentListSelect = `
SELECT
    ci.id, ci.name, ci.manufacturer, ci.model, ci.serial_number,
    ci.calibration_date, ci.expire_date, ci.active, ci.created_at, ci.updated_at,
    ci.created_by, ci.updated_by,
    (ci.expire_date IS NOT NULL AND ci.expire_date < current_date) AS is_expired,
    (ci.expire_date - current_date)::int AS days_until_expiry,
    (SELECT count(*) FROM rtu.calibrations c WHERE c.instrument_id = ci.id)::bigint AS calibration_count,
    count(*) OVER ()::bigint AS total_count
FROM rtu.calibration_instruments ci
WHERE %s
ORDER BY %s %s NULLS LAST, ci.id %s
LIMIT %s OFFSET %s`

// List returns one page of instruments together with the total row count.
func (r *CalibrationInstrumentRepository) List(ctx context.Context, page httpx.Page, filter CalibrationInstrumentFilter) ([]CalibrationInstrumentListItem, int64, error) {
	a := &args{}
	conds := conditions{}

	if filter.Active != nil {
		conds = append(conds, "ci.active = "+a.add(*filter.Active))
	}
	if filter.Manufacturer != nil {
		conds = append(conds, "ci.manufacturer = "+a.add(*filter.Manufacturer))
	}
	if filter.Expired != nil {
		if *filter.Expired {
			conds = append(conds, "ci.expire_date IS NOT NULL AND ci.expire_date < current_date")
		} else {
			conds = append(conds, "(ci.expire_date IS NULL OR ci.expire_date >= current_date)")
		}
	}
	if filter.ExpiringBefore != nil {
		conds = append(conds, "ci.expire_date IS NOT NULL AND ci.expire_date <= "+a.add(*filter.ExpiringBefore))
	}
	if page.Search != nil {
		p := a.add(likePattern(*page.Search))
		conds = append(conds, fmt.Sprintf(
			`(ci.name ILIKE %s ESCAPE '\' OR ci.serial_number ILIKE %s ESCAPE '\' OR ci.manufacturer ILIKE %s ESCAPE '\' OR ci.model ILIKE %s ESCAPE '\')`,
			p, p, p, p,
		))
	}

	query := fmt.Sprintf(instrumentListSelect,
		conds.where(), page.SortSQL, page.Order, page.Order,
		a.add(page.RowLimit()), a.add(page.Offset()),
	)

	rows, err := r.pool.Query(ctx, query, a.values...)
	if err != nil {
		return nil, 0, db.Translate(err)
	}

	items, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[CalibrationInstrumentListItem])
	if err != nil {
		return nil, 0, db.Translate(err)
	}

	var total int64
	if len(items) > 0 {
		total = items[0].TotalCount
	}
	return items, total, nil
}

// Get returns a single instrument by id.
func (r *CalibrationInstrumentRepository) Get(ctx context.Context, id uuid.UUID) (sqlc.CalibrationInstrument, error) {
	instrument, err := r.q.GetCalibrationInstrument(ctx, id)
	if err != nil {
		return sqlc.CalibrationInstrument{}, db.Translate(err, db.WithNotFound(httpx.ErrInstrumentNotFnd))
	}
	return instrument, nil
}

// Create inserts an instrument.
func (r *CalibrationInstrumentRepository) Create(ctx context.Context, arg sqlc.CreateCalibrationInstrumentParams) (sqlc.CalibrationInstrument, error) {
	arg.CreatedBy, arg.UpdatedBy = createAudit(ctx)
	instrument, err := r.q.CreateCalibrationInstrument(ctx, arg)
	if err != nil {
		return sqlc.CalibrationInstrument{}, db.Translate(err, db.Options{Constraints: instrumentConstraints})
	}
	return instrument, nil
}

// Update applies a partial update.
func (r *CalibrationInstrumentRepository) Update(ctx context.Context, arg sqlc.UpdateCalibrationInstrumentParams) (sqlc.CalibrationInstrument, error) {
	arg.UpdatedBy = updateAudit(ctx)
	instrument, err := r.q.UpdateCalibrationInstrument(ctx, arg)
	if err != nil {
		return sqlc.CalibrationInstrument{}, db.Translate(err, db.Options{
			NotFound:    &httpx.ErrInstrumentNotFnd,
			Constraints: instrumentConstraints,
		})
	}
	return instrument, nil
}

// SetActive soft-deletes or restores an instrument.
func (r *CalibrationInstrumentRepository) SetActive(ctx context.Context, id uuid.UUID, active bool) (sqlc.CalibrationInstrument, error) {
	instrument, err := r.q.SetCalibrationInstrumentActive(ctx, sqlc.SetCalibrationInstrumentActiveParams{
		ID: id, Active: active, UpdatedBy: updateAudit(ctx),
	})
	if err != nil {
		return sqlc.CalibrationInstrument{}, db.Translate(err, db.WithNotFound(httpx.ErrInstrumentNotFnd))
	}
	return instrument, nil
}

// Delete removes an instrument permanently.
func (r *CalibrationInstrumentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	affected, err := r.q.DeleteCalibrationInstrument(ctx, id)
	if err != nil {
		return db.Translate(err, db.Options{Constraints: instrumentDeleteConstraints})
	}
	if affected == 0 {
		return httpx.Err(httpx.ErrInstrumentNotFnd)
	}
	return nil
}

// Usability returns the fields needed to decide whether an instrument may be
// used for a new calibration.
func (r *CalibrationInstrumentRepository) Usability(ctx context.Context, id uuid.UUID) (active bool, expireDate *httpx.Date, err error) {
	row, err := r.q.GetCalibrationInstrumentUsability(ctx, id)
	if err != nil {
		return false, nil, db.Translate(err, db.WithNotFound(httpx.ErrInstrumentNotFnd))
	}
	return row.Active, row.ExpireDate, nil
}
