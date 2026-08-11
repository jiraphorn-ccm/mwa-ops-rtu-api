package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rtu-api/internal/db"
	"github.com/rtu-api/internal/db/sqlc"
	"github.com/rtu-api/internal/httpx"
)

// ChecklistItemRepository reads and writes rtu.checklist_items. The list is
// short (the standard 13-item PM checklist) so it is returned unpaginated,
// ordered by sort_order.
type ChecklistItemRepository struct {
	pool *pgxpool.Pool
	q    *sqlc.Queries
}

var checklistItemConstraints = db.Constraints{
	"uk_checklist_items_code": httpx.ErrChecklistItemCodeDup,
}

// List returns every checklist item, ordered by sort_order.
func (r *ChecklistItemRepository) List(ctx context.Context, active *bool) ([]sqlc.ChecklistItem, error) {
	items, err := r.q.ListChecklistItems(ctx, active)
	if err != nil {
		return nil, db.Translate(err)
	}
	return items, nil
}

// Get returns a single checklist item by id.
func (r *ChecklistItemRepository) Get(ctx context.Context, id uuid.UUID) (sqlc.ChecklistItem, error) {
	item, err := r.q.GetChecklistItem(ctx, id)
	if err != nil {
		return sqlc.ChecklistItem{}, db.Translate(err, db.WithNotFound(httpx.ErrChecklistItemNotFnd))
	}
	return item, nil
}

// Create inserts a checklist item.
func (r *ChecklistItemRepository) Create(ctx context.Context, arg sqlc.CreateChecklistItemParams) (sqlc.ChecklistItem, error) {
	arg.CreatedBy, arg.UpdatedBy = createAudit(ctx)
	item, err := r.q.CreateChecklistItem(ctx, arg)
	if err != nil {
		return sqlc.ChecklistItem{}, db.Translate(err, db.Options{Constraints: checklistItemConstraints})
	}
	return item, nil
}

// Update applies a partial update.
func (r *ChecklistItemRepository) Update(ctx context.Context, arg sqlc.UpdateChecklistItemParams) (sqlc.ChecklistItem, error) {
	arg.UpdatedBy = updateAudit(ctx)
	item, err := r.q.UpdateChecklistItem(ctx, arg)
	if err != nil {
		return sqlc.ChecklistItem{}, db.Translate(err, db.Options{
			NotFound:    &httpx.ErrChecklistItemNotFnd,
			Constraints: checklistItemConstraints,
		})
	}
	return item, nil
}

// SetActive soft-deletes or restores a checklist item.
func (r *ChecklistItemRepository) SetActive(ctx context.Context, id uuid.UUID, active bool) (sqlc.ChecklistItem, error) {
	item, err := r.q.SetChecklistItemActive(ctx, sqlc.SetChecklistItemActiveParams{
		ID: id, Active: active, UpdatedBy: updateAudit(ctx),
	})
	if err != nil {
		return sqlc.ChecklistItem{}, db.Translate(err, db.WithNotFound(httpx.ErrChecklistItemNotFnd))
	}
	return item, nil
}

// Delete removes a checklist item permanently.
func (r *ChecklistItemRepository) Delete(ctx context.Context, id uuid.UUID) error {
	affected, err := r.q.DeleteChecklistItem(ctx, id)
	if err != nil {
		return db.Translate(err, db.Options{Constraints: db.Constraints{
			"fk_checklist_results_item": httpx.ErrReferenced,
		}})
	}
	if affected == 0 {
		return httpx.Err(httpx.ErrChecklistItemNotFnd)
	}
	return nil
}
