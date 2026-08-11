package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rtu-api/internal/db"
	"github.com/rtu-api/internal/db/sqlc"
	"github.com/rtu-api/internal/httpx"
)

// WoApprovalRepository reads and writes rtu.wo_approvals. The workflow that
// a decision triggers (transition to COMPLETED/CONDITIONAL, opening a new
// round, spawning/reusing a CM work order) is orchestrated by ApprovalService
// using WorkOrderRepository — this repository only persists the decision
// itself.
type WoApprovalRepository struct {
	pool *pgxpool.Pool
	q    *sqlc.Queries
}

var woApprovalConstraints = db.Constraints{
	"uk_wo_approvals_round_id":       httpx.ErrApprovalRoundAlreadyReviewed,
	"fk_wo_approvals_work_order":     httpx.ErrWorkOrderNotFnd,
	"fk_wo_approvals_round":          httpx.ErrWorkOrderRoundNotFnd,
	"fk_wo_approvals_new_work_order": httpx.ErrWorkOrderNotFnd,
}

// Create records an approval decision for a round.
func (r *WoApprovalRepository) Create(ctx context.Context, arg sqlc.CreateWoApprovalParams) (sqlc.WoApproval, error) {
	arg.CreatedBy, arg.UpdatedBy = createAudit(ctx)
	approval, err := r.q.CreateWoApproval(ctx, arg)
	if err != nil {
		return sqlc.WoApproval{}, db.Translate(err, db.Options{Constraints: woApprovalConstraints})
	}
	return approval, nil
}

// Get returns a single approval decision.
func (r *WoApprovalRepository) Get(ctx context.Context, id uuid.UUID) (sqlc.WoApproval, error) {
	approval, err := r.q.GetWoApproval(ctx, id)
	if err != nil {
		return sqlc.WoApproval{}, db.Translate(err, db.WithNotFound(httpx.ErrApprovalNotFnd))
	}
	return approval, nil
}

// GetByRound returns the approval decision made for a specific round, if any.
func (r *WoApprovalRepository) GetByRound(ctx context.Context, roundID uuid.UUID) (sqlc.WoApproval, error) {
	approval, err := r.q.GetWoApprovalByRound(ctx, roundID)
	if err != nil {
		return sqlc.WoApproval{}, db.Translate(err, db.WithNotFound(httpx.ErrApprovalNotFnd))
	}
	return approval, nil
}

// ListByWorkOrder returns every approval decision made across all rounds of
// a work order, oldest first.
func (r *WoApprovalRepository) ListByWorkOrder(ctx context.Context, workOrderID uuid.UUID) ([]sqlc.WoApproval, error) {
	approvals, err := r.q.ListWoApprovalsByWorkOrder(ctx, workOrderID)
	if err != nil {
		return nil, db.Translate(err)
	}
	return approvals, nil
}

// ApprovalOutcome is the work-order side effect paired with recording a
// decision. Approval insert and these mutations commit atomically.
type ApprovalOutcome struct {
	NewStatus   string
	CloseWO     bool
	Rework      *ApprovalReworkOutcome
	ActivityLog *sqlc.CreateWorkOrderActivityLogParams
}

// ApprovalReworkOutcome opens round_no+1 and reassigns the work order.
type ApprovalReworkOutcome struct {
	AssignedTo uuid.UUID
	AssignedBy uuid.UUID
	AssignedAt time.Time
	ActorID    uuid.UUID
	FromStatus string
}

// DecideAndApply records the approval and applies outcome in one transaction.
func (r *WoApprovalRepository) DecideAndApply(
	ctx context.Context,
	approval sqlc.CreateWoApprovalParams,
	outcome ApprovalOutcome,
) (sqlc.WoApproval, error) {
	createdBy, updatedBy := createAudit(ctx)
	approval.CreatedBy, approval.UpdatedBy = createdBy, updatedBy

	var result sqlc.WoApproval
	err := db.InTx(ctx, r.pool, func(q *sqlc.Queries) error {
		var err error
		result, err = q.CreateWoApproval(ctx, approval)
		if err != nil {
			return db.Translate(err, db.Options{Constraints: woApprovalConstraints})
		}

		woID := approval.WorkOrderID
		if outcome.Rework != nil {
			rw := outcome.Rework
			nextNo, err := q.NextWorkOrderRoundNo(ctx, woID)
			if err != nil {
				return db.Translate(err)
			}
			round, err := q.CreateWorkOrderRound(ctx, sqlc.CreateWorkOrderRoundParams{
				WorkOrderID: woID,
				RoundNo:     nextNo,
				AssignedTo:  rw.AssignedTo,
				AssignedBy:  rw.AssignedBy,
				AssignedAt:  rw.AssignedAt,
				CreatedBy:   createdBy,
				UpdatedBy:   updatedBy,
			})
			if err != nil {
				return db.Translate(err)
			}
			if _, err := q.SetWorkOrderCurrentRound(ctx, sqlc.SetWorkOrderCurrentRoundParams{
				ID: woID, CurrentRoundID: round.ID, UpdatedBy: updatedBy,
			}); err != nil {
				return db.Translate(err)
			}
			wo, err := q.UpdateWorkOrderStatus(ctx, sqlc.UpdateWorkOrderStatusParams{
				ID: woID, Status: outcome.NewStatus, UpdatedBy: updatedBy,
			})
			if err != nil {
				return db.Translate(err)
			}
			from := rw.FromStatus
			to := wo.Status
			if _, err := q.CreateWorkOrderActivityLog(ctx, sqlc.CreateWorkOrderActivityLogParams{
				WorkOrderID:      woID,
				WorkOrderRoundID: &round.ID,
				Action:           "ASSIGNED",
				FromStatus:       &from,
				ToStatus:         &to,
				ToAssignee:       &rw.AssignedTo,
				ActorID:          rw.ActorID,
			}); err != nil {
				return db.Translate(err)
			}
			return nil
		}

		params := sqlc.UpdateWorkOrderStatusParams{
			ID: woID, Status: outcome.NewStatus, UpdatedBy: updatedBy,
		}
		if outcome.CloseWO {
			now := time.Now()
			params.ClosedAt, params.ClosedAtDoUpdate = &now, true
		}
		if _, err := q.UpdateWorkOrderStatus(ctx, params); err != nil {
			return db.Translate(err)
		}
		if outcome.ActivityLog != nil {
			if _, err := q.CreateWorkOrderActivityLog(ctx, *outcome.ActivityLog); err != nil {
				return db.Translate(err)
			}
		}
		return nil
	})
	if err != nil {
		return sqlc.WoApproval{}, err
	}
	return result, nil
}
