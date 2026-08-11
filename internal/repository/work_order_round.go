package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/rtu-api/internal/db"
	"github.com/rtu-api/internal/db/sqlc"
	"github.com/rtu-api/internal/httpx"
)

// WorkOrderRoundRepository reads and writes rtu.work_order_rounds. Every
// mutation that changes the parent work order's status or assignee also
// writes the matching rtu.work_order_activity_logs row, in the same
// transaction, so the timeline in ListActivity always matches reality.
type WorkOrderRoundRepository struct {
	pool *pgxpool.Pool
	q    *sqlc.Queries
}

// Get returns a single round.
func (r *WorkOrderRoundRepository) Get(ctx context.Context, id uuid.UUID) (sqlc.WorkOrderRound, error) {
	round, err := r.q.GetWorkOrderRound(ctx, id)
	if err != nil {
		return sqlc.WorkOrderRound{}, db.Translate(err, db.WithNotFound(httpx.ErrWorkOrderRoundNotFnd))
	}
	return round, nil
}

// ListByWorkOrder returns every round of a work order, oldest first — the
// full "who did what, when" history of every visit/rework attempt.
func (r *WorkOrderRoundRepository) ListByWorkOrder(ctx context.Context, workOrderID uuid.UUID) ([]sqlc.WorkOrderRound, error) {
	rounds, err := r.q.ListWorkOrderRoundsByWorkOrder(ctx, workOrderID)
	if err != nil {
		return nil, db.Translate(err)
	}
	return rounds, nil
}

// Reassign changes the assignee of the round currently in progress. Only
// valid before check-in; once work has started a rejection must open a new
// round (see WorkOrderRepository.OpenNewRound) instead of mutating this one.
func (r *WorkOrderRoundRepository) Reassign(
	ctx context.Context, workOrderID, roundID uuid.UUID,
	fromAssignee, toAssignee, assignedBy uuid.UUID, assignedAt time.Time, actorID uuid.UUID,
) (sqlc.WorkOrderRound, error) {
	var round sqlc.WorkOrderRound

	err := db.InTx(ctx, r.pool, func(q *sqlc.Queries) error {
		updated, err := q.UpdateWorkOrderRoundAssignment(ctx, sqlc.UpdateWorkOrderRoundAssignmentParams{
			ID: roundID, AssignedTo: toAssignee, AssignedBy: assignedBy, AssignedAt: assignedAt,
			UpdatedBy: updateAudit(ctx),
		})
		if err != nil {
			return db.Translate(err, db.WithNotFound(httpx.ErrRoundAlreadyCheckedIn))
		}
		round = updated

		if _, err := q.CreateWorkOrderActivityLog(ctx, sqlc.CreateWorkOrderActivityLogParams{
			WorkOrderID:      workOrderID,
			WorkOrderRoundID: &roundID,
			Action:           "REASSIGNED",
			FromAssignee:     &fromAssignee,
			ToAssignee:       &toAssignee,
			ActorID:          actorID,
		}); err != nil {
			return db.Translate(err)
		}
		return nil
	})
	if err != nil {
		return sqlc.WorkOrderRound{}, db.Translate(err)
	}
	return round, nil
}

// CheckIn records a mobile check-in for the round and moves the parent work
// order to IN_PROGRESS.
func (r *WorkOrderRoundRepository) CheckIn(
	ctx context.Context, workOrderID, roundID uuid.UUID,
	checkInAt time.Time, lat, lng *decimal.Decimal, fromStatus string, actorID uuid.UUID,
) (sqlc.WorkOrderRound, sqlc.WorkOrder, error) {
	var (
		round sqlc.WorkOrderRound
		wo    sqlc.WorkOrder
	)

	err := db.InTx(ctx, r.pool, func(q *sqlc.Queries) error {
		updated, err := q.CheckInWorkOrderRound(ctx, sqlc.CheckInWorkOrderRoundParams{
			ID: roundID, CheckInAt: checkInAt, CheckInLat: lat, CheckInLng: lng,
			UpdatedBy: updateAudit(ctx),
		})
		if err != nil {
			return db.Translate(err, db.WithNotFound(httpx.ErrRoundAlreadyCheckedIn))
		}
		round = updated

		wo, err = q.UpdateWorkOrderStatus(ctx, sqlc.UpdateWorkOrderStatusParams{
			ID: workOrderID, Status: "IN_PROGRESS", UpdatedBy: updateAudit(ctx),
		})
		if err != nil {
			return db.Translate(err)
		}

		from := fromStatus
		to := wo.Status
		if _, err := q.CreateWorkOrderActivityLog(ctx, sqlc.CreateWorkOrderActivityLogParams{
			WorkOrderID:      workOrderID,
			WorkOrderRoundID: &roundID,
			Action:           "CHECKED_IN",
			FromStatus:       &from,
			ToStatus:         &to,
			ActorID:          actorID,
		}); err != nil {
			return db.Translate(err)
		}
		return nil
	})
	if err != nil {
		return sqlc.WorkOrderRound{}, sqlc.WorkOrder{}, db.Translate(err)
	}
	return round, wo, nil
}

// CheckOut records a mobile check-out for the round. The work order's status
// stays IN_PROGRESS until its PM/CM report is submitted (see MarkSubmitted).
func (r *WorkOrderRoundRepository) CheckOut(
	ctx context.Context, workOrderID, roundID uuid.UUID,
	checkOutAt time.Time, lat, lng *decimal.Decimal, actorID uuid.UUID,
) (sqlc.WorkOrderRound, error) {
	var round sqlc.WorkOrderRound

	err := db.InTx(ctx, r.pool, func(q *sqlc.Queries) error {
		updated, err := q.CheckOutWorkOrderRound(ctx, sqlc.CheckOutWorkOrderRoundParams{
			ID: roundID, CheckOutAt: checkOutAt, CheckOutLat: lat, CheckOutLng: lng,
			UpdatedBy: updateAudit(ctx),
		})
		if err != nil {
			return db.Translate(err, db.WithNotFound(httpx.ErrRoundNotCheckedIn))
		}
		round = updated

		if _, err := q.CreateWorkOrderActivityLog(ctx, sqlc.CreateWorkOrderActivityLogParams{
			WorkOrderID:      workOrderID,
			WorkOrderRoundID: &roundID,
			Action:           "CHECKED_OUT",
			ActorID:          actorID,
		}); err != nil {
			return db.Translate(err)
		}
		return nil
	})
	if err != nil {
		return sqlc.WorkOrderRound{}, db.Translate(err)
	}
	return round, nil
}

// MarkSubmitted stamps the round's submitted_at and moves the parent work
// order to PENDING_APPROVAL. Called by the PM/CM report services once a
// report is submitted for the round.
func (r *WorkOrderRoundRepository) MarkSubmitted(
	ctx context.Context, workOrderID, roundID uuid.UUID, submittedAt time.Time, fromStatus string, actorID uuid.UUID,
) (sqlc.WorkOrderRound, sqlc.WorkOrder, error) {
	var (
		round sqlc.WorkOrderRound
		wo    sqlc.WorkOrder
	)

	err := db.InTx(ctx, r.pool, func(q *sqlc.Queries) error {
		updated, err := q.SetWorkOrderRoundSubmitted(ctx, sqlc.SetWorkOrderRoundSubmittedParams{
			ID: roundID, SubmittedAt: &submittedAt, UpdatedBy: updateAudit(ctx),
		})
		if err != nil {
			return db.Translate(err, db.WithNotFound(httpx.ErrWorkOrderRoundNotFnd))
		}
		round = updated

		wo, err = q.UpdateWorkOrderStatus(ctx, sqlc.UpdateWorkOrderStatusParams{
			ID: workOrderID, Status: "PENDING_APPROVAL", UpdatedBy: updateAudit(ctx),
		})
		if err != nil {
			return db.Translate(err)
		}

		from := fromStatus
		to := wo.Status
		if _, err := q.CreateWorkOrderActivityLog(ctx, sqlc.CreateWorkOrderActivityLogParams{
			WorkOrderID:      workOrderID,
			WorkOrderRoundID: &roundID,
			Action:           "SUBMITTED",
			FromStatus:       &from,
			ToStatus:         &to,
			ActorID:          actorID,
		}); err != nil {
			return db.Translate(err)
		}
		return nil
	})
	if err != nil {
		return sqlc.WorkOrderRound{}, sqlc.WorkOrder{}, db.Translate(err)
	}
	return round, wo, nil
}
