package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/rtu-api/internal/db"
	"github.com/rtu-api/internal/db/sqlc"
)

// BeginWorkFromReportInput marks a work order as started when a technician
// saves a report draft (PM or CM) instead of calling check-in.
type BeginWorkFromReportInput struct {
	WorkOrderID uuid.UUID
	RoundID     uuid.UUID
	StartedAt   time.Time
	ActorID     uuid.UUID
	FromStatus  string
}

// BeginWorkFromReportQ moves ASSIGNED/PENDING → IN_PROGRESS, stamps
// work_order_rounds.check_in_at (started-at for UI), and logs STATUS_CHANGED.
// Idempotent when the work order is already IN_PROGRESS.
func BeginWorkFromReportQ(ctx context.Context, q *sqlc.Queries, in BeginWorkFromReportInput) error {
	switch in.FromStatus {
	case "ASSIGNED", "PENDING":
	default:
		return nil
	}

	_, updatedBy := createAudit(ctx)

	if _, err := q.CheckInWorkOrderRound(ctx, sqlc.CheckInWorkOrderRoundParams{
		ID:         in.RoundID,
		CheckInAt:  in.StartedAt,
		CheckInLat: nil,
		CheckInLng: nil,
		UpdatedBy:  updatedBy,
	}); err != nil {
		return db.Translate(err)
	}

	wo, err := q.UpdateWorkOrderStatus(ctx, sqlc.UpdateWorkOrderStatusParams{
		ID: in.WorkOrderID, Status: "IN_PROGRESS", UpdatedBy: updatedBy,
	})
	if err != nil {
		return db.Translate(err)
	}

	from := in.FromStatus
	to := wo.Status
	note := "Work started from report"
	if _, err := q.CreateWorkOrderActivityLog(ctx, sqlc.CreateWorkOrderActivityLogParams{
		WorkOrderID:      in.WorkOrderID,
		WorkOrderRoundID: &in.RoundID,
		Action:           "STATUS_CHANGED",
		FromStatus:       &from,
		ToStatus:         &to,
		Note:             &note,
		ActorID:          in.ActorID,
	}); err != nil {
		return db.Translate(err)
	}
	return nil
}
