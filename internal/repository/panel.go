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

// PanelRepository reads and writes rtu.panels.
type PanelRepository struct {
	pool *pgxpool.Pool
	q    *sqlc.Queries
}

// panelConstraints maps the constraints of rtu.panels onto business errors.
var panelConstraints = db.Constraints{
	"uk_panels_code": httpx.ErrPanelCodeExists,
}

// panelDeleteConstraints explains why a panel cannot be removed.
var panelDeleteConstraints = db.Constraints{
	"fk_panel_devices_panel": httpx.ErrPanelInUse,
}

var panelSortable = httpx.Sortable{
	"code":       "p.code",
	"location":   "p.location",
	"active":     "p.active",
	"created_at": "p.created_at",
	"updated_at": "p.updated_at",
}

// PanelSortable lists the sort keys accepted by the panel list endpoint.
func PanelSortable() httpx.Sortable { return panelSortable }

// PanelFilter narrows a panel list query.
type PanelFilter struct {
	Active      *bool
	HasLocation *bool
	CreatedFrom *time.Time
	CreatedTo   *time.Time
}

// PanelDetail is a panel row with computed operational status for API responses.
// OperationalStatus is never stored — see panelStatusExprSQL. The db tag
// still reads the "status" SQL alias; only the JSON name is domain-specific.
type PanelDetail struct {
	sqlc.Panel
	OperationalStatus string `db:"status" json:"operational_status"`
}

// PanelListItem is a panel enriched with the counters shown in list views.
type PanelListItem struct {
	sqlc.Panel
	OperationalStatus string `db:"status" json:"operational_status"`
	DeviceCount       int64  `db:"device_count" json:"device_count"`
	TotalCount        int64  `db:"total_count" json:"-"`
}

const panelListSelect = `
SELECT
    p.id, p.code, p.location, p.latitude, p.longitude, p.active,
    p.created_at, p.updated_at, p.created_by, p.updated_by,
    (` + panelStatusExprSQL + `) AS status,
    (SELECT count(*) FROM rtu.panel_devices pd WHERE pd.panel_id = p.id)::bigint AS device_count,
    count(*) OVER ()::bigint AS total_count
FROM rtu.panels p
WHERE %s
ORDER BY %s %s, p.id %s
LIMIT %s OFFSET %s`

const panelDetailSelect = `
SELECT
    p.id, p.code, p.location, p.latitude, p.longitude, p.active,
    p.created_at, p.updated_at, p.created_by, p.updated_by,
    (` + panelStatusExprSQL + `) AS status
FROM rtu.panels p
WHERE %s`

// List returns one page of panels together with the total row count.
func (r *PanelRepository) List(ctx context.Context, page httpx.Page, filter PanelFilter) ([]PanelListItem, int64, error) {
	a := &args{}
	conds := conditions{}

	if filter.Active != nil {
		conds = append(conds, "p.active = "+a.add(*filter.Active))
	}
	if filter.HasLocation != nil {
		if *filter.HasLocation {
			conds = append(conds, "p.location IS NOT NULL")
		} else {
			conds = append(conds, "p.location IS NULL")
		}
	}
	if filter.CreatedFrom != nil {
		conds = append(conds, "p.created_at >= "+a.add(*filter.CreatedFrom))
	}
	if filter.CreatedTo != nil {
		conds = append(conds, "p.created_at <= "+a.add(*filter.CreatedTo))
	}
	if page.Search != nil {
		p := a.add(likePattern(*page.Search))
		conds = append(conds, fmt.Sprintf(`(p.code ILIKE %s ESCAPE '\' OR p.location ILIKE %s ESCAPE '\')`, p, p))
	}

	query := fmt.Sprintf(panelListSelect,
		conds.where(), page.SortSQL, page.Order, page.Order,
		a.add(page.RowLimit()), a.add(page.Offset()),
	)

	rows, err := r.pool.Query(ctx, query, a.values...)
	if err != nil {
		return nil, 0, db.Translate(err)
	}

	items, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[PanelListItem])
	if err != nil {
		return nil, 0, db.Translate(err)
	}

	var total int64
	if len(items) > 0 {
		total = items[0].TotalCount
	}
	return items, total, nil
}

// Get returns a single panel by id with computed status.
func (r *PanelRepository) Get(ctx context.Context, id uuid.UUID) (PanelDetail, error) {
	query := fmt.Sprintf(panelDetailSelect, "p.id = $1")
	row, err := r.pool.Query(ctx, query, id)
	if err != nil {
		return PanelDetail{}, db.Translate(err)
	}
	panel, err := pgx.CollectOneRow(row, pgx.RowToStructByNameLax[PanelDetail])
	if err != nil {
		return PanelDetail{}, db.Translate(err, db.WithNotFound(httpx.ErrPanelNotFound))
	}
	return panel, nil
}

// GetByCode returns a single panel by its business code with computed status.
func (r *PanelRepository) GetByCode(ctx context.Context, code string) (PanelDetail, error) {
	query := fmt.Sprintf(panelDetailSelect, "p.code = $1")
	row, err := r.pool.Query(ctx, query, code)
	if err != nil {
		return PanelDetail{}, db.Translate(err)
	}
	panel, err := pgx.CollectOneRow(row, pgx.RowToStructByNameLax[PanelDetail])
	if err != nil {
		return PanelDetail{}, db.Translate(err, db.WithNotFound(httpx.ErrPanelNotFound))
	}
	return panel, nil
}

func (r *PanelRepository) detailAfterWrite(ctx context.Context, panel sqlc.Panel) (PanelDetail, error) {
	return r.Get(ctx, panel.ID)
}

// Create inserts a panel.
func (r *PanelRepository) Create(ctx context.Context, arg sqlc.CreatePanelParams) (PanelDetail, error) {
	arg.CreatedBy, arg.UpdatedBy = createAudit(ctx)
	panel, err := r.q.CreatePanel(ctx, arg)
	if err != nil {
		return PanelDetail{}, db.Translate(err, db.Options{Constraints: panelConstraints})
	}
	return r.detailAfterWrite(ctx, panel)
}

// Update applies a partial update.
func (r *PanelRepository) Update(ctx context.Context, arg sqlc.UpdatePanelParams) (PanelDetail, error) {
	arg.UpdatedBy = updateAudit(ctx)
	panel, err := r.q.UpdatePanel(ctx, arg)
	if err != nil {
		return PanelDetail{}, db.Translate(err, db.Options{
			NotFound:    &httpx.ErrPanelNotFound,
			Constraints: panelConstraints,
		})
	}
	return r.detailAfterWrite(ctx, panel)
}

// UpdatePmDates writes the denormalized last_pm_date / next_pm_date after a
// PM work order is completed (APPROVED or APPROVED_CONDITION).
func (r *PanelRepository) UpdatePmDates(ctx context.Context, id uuid.UUID, lastPm, nextPm httpx.Date) (PanelDetail, error) {
	panel, err := r.q.UpdatePanelPmDates(ctx, sqlc.UpdatePanelPmDatesParams{
		ID:         id,
		LastPmDate: lastPm,
		NextPmDate: nextPm,
		UpdatedBy:  updateAudit(ctx),
	})
	if err != nil {
		return PanelDetail{}, db.Translate(err, db.WithNotFound(httpx.ErrPanelNotFound))
	}
	return r.detailAfterWrite(ctx, panel)
}

// SetActive soft-deletes or restores a panel.
func (r *PanelRepository) SetActive(ctx context.Context, id uuid.UUID, active bool) (PanelDetail, error) {
	panel, err := r.q.SetPanelActive(ctx, sqlc.SetPanelActiveParams{
		ID: id, Active: active, UpdatedBy: updateAudit(ctx),
	})
	if err != nil {
		return PanelDetail{}, db.Translate(err, db.WithNotFound(httpx.ErrPanelNotFound))
	}
	return r.detailAfterWrite(ctx, panel)
}

// Delete removes a panel permanently.
func (r *PanelRepository) Delete(ctx context.Context, id uuid.UUID) error {
	affected, err := r.q.DeletePanel(ctx, id)
	if err != nil {
		return db.Translate(err, db.Options{Constraints: panelDeleteConstraints})
	}
	if affected == 0 {
		return httpx.Err(httpx.ErrPanelNotFound)
	}
	return nil
}

// Exists reports whether a panel id is known.
func (r *PanelRepository) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	found, err := r.q.PanelExists(ctx, id)
	if err != nil {
		return false, db.Translate(err)
	}
	return found, nil
}
