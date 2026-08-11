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

// EngineerRepository reads and writes rtu.engineers.
type EngineerRepository struct {
	pool *pgxpool.Pool
	q    *sqlc.Queries
}

var engineerSortable = httpx.Sortable{
	"full_name":  "e.full_name",
	"license_no": "e.license_no",
	"active":     "e.active",
	"created_at": "e.created_at",
	"updated_at": "e.updated_at",
}

// EngineerSortable lists the sort keys accepted by the list endpoint.
func EngineerSortable() httpx.Sortable { return engineerSortable }

// EngineerFilter narrows an engineer list query.
type EngineerFilter struct {
	Active *bool
}

const engineerListSelect = `
SELECT
    e.id, e.full_name, e.license_no, e.position, e.active,
    e.created_at, e.updated_at, e.created_by, e.updated_by,
    count(*) OVER ()::bigint AS total_count
FROM rtu.engineers e
WHERE %s
ORDER BY %s %s, e.id %s
LIMIT %s OFFSET %s`

// EngineerListItem is an engineer row with the total row count of its page.
type EngineerListItem struct {
	sqlc.Engineer
	TotalCount int64 `db:"total_count" json:"-"`
}

// List returns one page of engineers together with the total row count.
func (r *EngineerRepository) List(ctx context.Context, page httpx.Page, filter EngineerFilter) ([]EngineerListItem, int64, error) {
	a := &args{}
	conds := conditions{}

	if filter.Active != nil {
		conds = append(conds, "e.active = "+a.add(*filter.Active))
	}
	if page.Search != nil {
		p := a.add(likePattern(*page.Search))
		conds = append(conds, fmt.Sprintf(`(e.full_name ILIKE %s ESCAPE '\' OR e.license_no ILIKE %s ESCAPE '\')`, p, p))
	}

	query := fmt.Sprintf(engineerListSelect,
		conds.where(), page.SortSQL, page.Order, page.Order,
		a.add(page.RowLimit()), a.add(page.Offset()),
	)

	rows, err := r.pool.Query(ctx, query, a.values...)
	if err != nil {
		return nil, 0, db.Translate(err)
	}

	items, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[EngineerListItem])
	if err != nil {
		return nil, 0, db.Translate(err)
	}

	var total int64
	if len(items) > 0 {
		total = items[0].TotalCount
	}
	return items, total, nil
}

// Get returns a single engineer by id.
func (r *EngineerRepository) Get(ctx context.Context, id uuid.UUID) (sqlc.Engineer, error) {
	engineer, err := r.q.GetEngineer(ctx, id)
	if err != nil {
		return sqlc.Engineer{}, db.Translate(err, db.WithNotFound(httpx.ErrEngineerNotFnd))
	}
	return engineer, nil
}

// Create inserts an engineer.
func (r *EngineerRepository) Create(ctx context.Context, arg sqlc.CreateEngineerParams) (sqlc.Engineer, error) {
	arg.CreatedBy, arg.UpdatedBy = createAudit(ctx)
	engineer, err := r.q.CreateEngineer(ctx, arg)
	if err != nil {
		return sqlc.Engineer{}, db.Translate(err)
	}
	return engineer, nil
}

// Update applies a partial update.
func (r *EngineerRepository) Update(ctx context.Context, arg sqlc.UpdateEngineerParams) (sqlc.Engineer, error) {
	arg.UpdatedBy = updateAudit(ctx)
	engineer, err := r.q.UpdateEngineer(ctx, arg)
	if err != nil {
		return sqlc.Engineer{}, db.Translate(err, db.WithNotFound(httpx.ErrEngineerNotFnd))
	}
	return engineer, nil
}

// SetActive soft-deletes or restores an engineer.
func (r *EngineerRepository) SetActive(ctx context.Context, id uuid.UUID, active bool) (sqlc.Engineer, error) {
	engineer, err := r.q.SetEngineerActive(ctx, sqlc.SetEngineerActiveParams{
		ID: id, Active: active, UpdatedBy: updateAudit(ctx),
	})
	if err != nil {
		return sqlc.Engineer{}, db.Translate(err, db.WithNotFound(httpx.ErrEngineerNotFnd))
	}
	return engineer, nil
}

// Delete removes an engineer permanently.
func (r *EngineerRepository) Delete(ctx context.Context, id uuid.UUID) error {
	affected, err := r.q.DeleteEngineer(ctx, id)
	if err != nil {
		return db.Translate(err, db.Options{Constraints: db.Constraints{
			"fk_pm_reports_engineer": httpx.ErrReferenced,
		}})
	}
	if affected == 0 {
		return httpx.Err(httpx.ErrEngineerNotFnd)
	}
	return nil
}
