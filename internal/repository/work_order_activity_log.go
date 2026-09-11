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

// WorkOrderActivityLogRepository reads rtu.work_order_activity_logs. Rows are
// written as part of the work order / round transactions that cause them
// (see WorkOrderRepository and WorkOrderRoundRepository); this repository
// only exposes the read side for the timeline endpoint.
type WorkOrderActivityLogRepository struct {
	pool *pgxpool.Pool
	q    *sqlc.Queries
}

// Create writes a single activity log row directly, for events that are not
// already covered by a WorkOrderRepository/WorkOrderRoundRepository
// transaction (e.g. CM_SPAWNED, recorded on the originating PM work order
// after ApprovalService creates or reuses the CM work order).
func (r *WorkOrderActivityLogRepository) Create(ctx context.Context, arg sqlc.CreateWorkOrderActivityLogParams) (sqlc.WorkOrderActivityLog, error) {
	log, err := r.q.CreateWorkOrderActivityLog(ctx, arg)
	if err != nil {
		return sqlc.WorkOrderActivityLog{}, db.Translate(err)
	}
	return log, nil
}

// ListByWorkOrder returns the full status/assignment timeline of a work
// order, oldest first.
func (r *WorkOrderActivityLogRepository) ListByWorkOrder(ctx context.Context, workOrderID uuid.UUID) ([]sqlc.WorkOrderActivityLog, error) {
	logs, err := r.q.ListWorkOrderActivityLogs(ctx, workOrderID)
	if err != nil {
		return nil, db.Translate(err)
	}
	return logs, nil
}

// PanelRepairActivityItem is one audit row for panel repair timeline UIs.
type PanelRepairActivityItem struct {
	sqlc.WorkOrderActivityLog
	WorkOrderNo   string `db:"work_order_no" json:"work_order_no"`
	WorkOrderType string `db:"work_order_type" json:"work_order_type"`
	TotalCount    int64  `db:"total_count" json:"-"`
}

const panelRepairActivitySelect = `
SELECT
    wal.id, wal.work_order_id, wal.work_order_round_id, wal.action,
    wal.from_status, wal.to_status, wal.from_assignee, wal.to_assignee,
    wal.note, wal.actor_id, wal.created_at,
    wo.work_order_no, wo.work_order_type,
    count(*) OVER ()::bigint AS total_count
FROM rtu.work_order_activity_logs wal
INNER JOIN rtu.work_orders wo ON wo.id = wal.work_order_id
WHERE wo.panel_id = $1
  AND (
    wo.work_order_type = 'CM'
    OR wal.action IN ('CM_SPAWNED', 'ONSITE_CM_OPENED')
  )
ORDER BY wal.created_at DESC, wal.id DESC
LIMIT $2 OFFSET $3`

// ListRepairActivityByPanel returns repair-related activity on a panel, newest first.
func (r *WorkOrderActivityLogRepository) ListRepairActivityByPanel(ctx context.Context, panelID uuid.UUID, page httpx.Page) ([]PanelRepairActivityItem, int64, error) {
	rows, err := r.pool.Query(ctx, panelRepairActivitySelect, panelID, page.RowLimit(), page.Offset())
	if err != nil {
		return nil, 0, db.Translate(err)
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[PanelRepairActivityItem])
	if err != nil {
		return nil, 0, fmt.Errorf("collect panel repair activity: %w", err)
	}
	var total int64
	if len(items) > 0 {
		total = items[0].TotalCount
	}
	return items, total, nil
}
