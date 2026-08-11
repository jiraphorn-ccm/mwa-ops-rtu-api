package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rtu-api/internal/db"
	"github.com/rtu-api/internal/db/sqlc"
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
