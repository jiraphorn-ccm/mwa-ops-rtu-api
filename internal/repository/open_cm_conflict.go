package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/rtu-api/internal/db"
)

// OpenCmConflictError is returned when an open CM work order already covers the
// same panel/topic (and device when filtered).
type OpenCmConflictError struct {
	Conflict OpenCmWorkOrderSummary
}

func (e *OpenCmConflictError) Error() string {
	return fmt.Sprintf("open CM work order %s already covers this problem topic", e.Conflict.WorkOrderNo)
}

type pgxQueryer interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func queryOpenCmForPanel(ctx context.Context, q pgxQueryer, panelID uuid.UUID, filter OpenCmWorkOrderFilter) ([]OpenCmWorkOrderSummary, error) {
	a := &args{}
	conds := buildOpenCmWorkOrderConditions(a, panelID, filter)

	query := fmt.Sprintf(openCmWorkOrderSelect, conds.where())
	rows, err := q.Query(ctx, query, a.values...)
	if err != nil {
		return nil, db.Translate(err)
	}
	defer rows.Close()

	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[OpenCmWorkOrderSummary])
	if err != nil {
		return nil, db.Translate(err)
	}
	return items, nil
}

func firstOpenCmConflict(ctx context.Context, q pgxQueryer, panelID uuid.UUID, filter *OpenCmWorkOrderFilter) (*OpenCmWorkOrderSummary, error) {
	if filter == nil || filter.ProblemTopicID == nil {
		return nil, nil
	}
	items, err := queryOpenCmForPanel(ctx, q, panelID, *filter)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	return &items[0], nil
}

// LockPanelCmWrites serializes CM duplicate checks and writes for one panel
// inside the current transaction.
func LockPanelCmWrites(ctx context.Context, tx pgx.Tx, panelID uuid.UUID) error {
	_, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtext($1::text))", panelID.String())
	if err != nil {
		return db.Translate(err)
	}
	return nil
}

// EnsureNoOpenCmConflict returns OpenCmConflictError when another open CM on
// the panel already covers the same problem topic.
func EnsureNoOpenCmConflict(ctx context.Context, q pgxQueryer, panelID uuid.UUID, filter OpenCmWorkOrderFilter) error {
	conflict, err := firstOpenCmConflict(ctx, q, panelID, &filter)
	if err != nil {
		return err
	}
	if conflict != nil {
		return &OpenCmConflictError{Conflict: *conflict}
	}
	return nil
}
