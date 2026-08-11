package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rtu-api/internal/db"
	"github.com/rtu-api/internal/db/sqlc"
	"github.com/rtu-api/internal/httpx"
)

// DeviceModelRepository reads and writes rtu.device_models.
type DeviceModelRepository struct {
	pool *pgxpool.Pool
	q    *sqlc.Queries
}

var deviceModelConstraints = db.Constraints{
	"uk_device_model_code": httpx.ErrDeviceModelCodeDup,
}

var deviceModelDeleteConstraints = db.Constraints{
	"fk_panel_devices_device_model": httpx.ErrDeviceModelInUse,
}

var deviceModelSortable = httpx.Sortable{
	"code":         "dm.code",
	"name":         "dm.name",
	"manufacturer": "dm.manufacturer",
	"model":        "dm.model",
	"active":       "dm.active",
	"created_at":   "dm.created_at",
	"updated_at":   "dm.updated_at",
}

// DeviceModelSortable lists the sort keys accepted by the list endpoint.
func DeviceModelSortable() httpx.Sortable { return deviceModelSortable }

// DeviceModelFilter narrows a device model list query.
type DeviceModelFilter struct {
	Active       *bool
	Manufacturer *string
}

// DeviceModelListItem is a device model enriched with its usage counter.
type DeviceModelListItem struct {
	sqlc.DeviceModel
	DeviceCount int64 `db:"device_count" json:"device_count"`
	TotalCount  int64 `db:"total_count" json:"-"`
}

const deviceModelListSelect = `
SELECT
    dm.id, dm.code, dm.name, dm.manufacturer, dm.model, dm.description,
    dm.active, dm.created_at, dm.updated_at, dm.created_by, dm.updated_by,
    (SELECT count(*) FROM rtu.panel_devices pd WHERE pd.device_model_id = dm.id)::bigint AS device_count,
    count(*) OVER ()::bigint AS total_count
FROM rtu.device_models dm
WHERE %s
ORDER BY %s %s, dm.id %s
LIMIT %s OFFSET %s`

// List returns one page of device models together with the total row count.
func (r *DeviceModelRepository) List(ctx context.Context, page httpx.Page, filter DeviceModelFilter) ([]DeviceModelListItem, int64, error) {
	a := &args{}
	conds := conditions{}

	if filter.Active != nil {
		conds = append(conds, "dm.active = "+a.add(*filter.Active))
	}
	if filter.Manufacturer != nil {
		conds = append(conds, "dm.manufacturer = "+a.add(*filter.Manufacturer))
	}
	if page.Search != nil {
		p := a.add(likePattern(*page.Search))
		conds = append(conds, fmt.Sprintf(
			`(dm.code ILIKE %s ESCAPE '\' OR dm.name ILIKE %s ESCAPE '\' OR dm.manufacturer ILIKE %s ESCAPE '\' OR dm.model ILIKE %s ESCAPE '\')`,
			p, p, p, p,
		))
	}

	query := fmt.Sprintf(deviceModelListSelect,
		conds.where(), page.SortSQL, page.Order, page.Order,
		a.add(page.RowLimit()), a.add(page.Offset()),
	)

	rows, err := r.pool.Query(ctx, query, a.values...)
	if err != nil {
		return nil, 0, db.Translate(err)
	}

	items, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[DeviceModelListItem])
	if err != nil {
		return nil, 0, db.Translate(err)
	}

	var total int64
	if len(items) > 0 {
		total = items[0].TotalCount
	}
	return items, total, nil
}

// Get returns a single device model by id.
func (r *DeviceModelRepository) Get(ctx context.Context, id uuid.UUID) (sqlc.DeviceModel, error) {
	model, err := r.q.GetDeviceModel(ctx, id)
	if err != nil {
		return sqlc.DeviceModel{}, db.Translate(err, db.WithNotFound(httpx.ErrDeviceModelNotFnd))
	}
	return model, nil
}

// GetByCode returns a single device model by its business code.
func (r *DeviceModelRepository) GetByCode(ctx context.Context, code string) (sqlc.DeviceModel, error) {
	model, err := r.q.GetDeviceModelByCode(ctx, code)
	if err != nil {
		return sqlc.DeviceModel{}, db.Translate(err, db.WithNotFound(httpx.ErrDeviceModelNotFnd))
	}
	return model, nil
}

// Create inserts a device model.
func (r *DeviceModelRepository) Create(ctx context.Context, arg sqlc.CreateDeviceModelParams) (sqlc.DeviceModel, error) {
	arg.CreatedBy, arg.UpdatedBy = createAudit(ctx)
	model, err := r.q.CreateDeviceModel(ctx, arg)
	if err != nil {
		return sqlc.DeviceModel{}, db.Translate(err, db.Options{Constraints: deviceModelConstraints})
	}
	return model, nil
}

// Update applies a partial update.
func (r *DeviceModelRepository) Update(ctx context.Context, arg sqlc.UpdateDeviceModelParams) (sqlc.DeviceModel, error) {
	arg.UpdatedBy = updateAudit(ctx)
	model, err := r.q.UpdateDeviceModel(ctx, arg)
	if err != nil {
		return sqlc.DeviceModel{}, db.Translate(err, db.Options{
			NotFound:    &httpx.ErrDeviceModelNotFnd,
			Constraints: deviceModelConstraints,
		})
	}
	return model, nil
}

// SetActive soft-deletes or restores a device model.
func (r *DeviceModelRepository) SetActive(ctx context.Context, id uuid.UUID, active bool) (sqlc.DeviceModel, error) {
	model, err := r.q.SetDeviceModelActive(ctx, sqlc.SetDeviceModelActiveParams{
		ID: id, Active: active, UpdatedBy: updateAudit(ctx),
	})
	if err != nil {
		return sqlc.DeviceModel{}, db.Translate(err, db.WithNotFound(httpx.ErrDeviceModelNotFnd))
	}
	return model, nil
}

// Delete removes a device model permanently.
func (r *DeviceModelRepository) Delete(ctx context.Context, id uuid.UUID) error {
	affected, err := r.q.DeleteDeviceModel(ctx, id)
	if err != nil {
		return db.Translate(err, db.Options{Constraints: deviceModelDeleteConstraints})
	}
	if affected == 0 {
		return httpx.Err(httpx.ErrDeviceModelNotFnd)
	}
	return nil
}

// Exists reports whether a device model id is known.
func (r *DeviceModelRepository) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	found, err := r.q.DeviceModelExists(ctx, id)
	if err != nil {
		return false, db.Translate(err)
	}
	return found, nil
}
