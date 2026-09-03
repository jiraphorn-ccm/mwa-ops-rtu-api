package repository

import (
	"context"
	"errors"
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

// OpenCmDuplicateCheck validates CM create/update against open panel+topic pairs.
type OpenCmDuplicateCheck struct {
	TopicIDs           []uuid.UUID
	ExcludeWorkOrderID *uuid.UUID
}

type pgxQueryer interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

const openCmConflictByTopicsSelect = `
SELECT
    wo.id AS work_order_id,
    wo.work_order_no,
    wo.status,
    wopt.problem_topic_id,
    pt.code AS problem_topic_code,
    pt.name AS problem_topic_name
FROM rtu.work_orders wo
JOIN rtu.work_order_problem_topics wopt ON wopt.work_order_id = wo.id
JOIN rtu.problem_topics pt ON pt.id = wopt.problem_topic_id
WHERE wo.panel_id = $1
  AND wo.work_order_type = 'CM'
  AND wo.active = true
  AND wo.status IN ('ASSIGNED', 'IN_PROGRESS', 'PENDING', 'PENDING_APPROVAL')
  AND wopt.problem_topic_id = ANY($2::uuid[])
  AND ($3::uuid IS NULL OR wo.id <> $3)
ORDER BY wo.created_at DESC, wo.id DESC
LIMIT 1`

type openCmConflictRow struct {
	WorkOrderID      uuid.UUID  `db:"work_order_id"`
	WorkOrderNo      string     `db:"work_order_no"`
	Status           string     `db:"status"`
	ProblemTopicID   *uuid.UUID `db:"problem_topic_id"`
	ProblemTopicCode *string    `db:"problem_topic_code"`
	ProblemTopicName *string    `db:"problem_topic_name"`
}

func openCmConflictRowToSummary(row openCmConflictRow) OpenCmWorkOrderSummary {
	summary := OpenCmWorkOrderSummary{
		WorkOrderID:      row.WorkOrderID,
		WorkOrderNo:      row.WorkOrderNo,
		Status:           row.Status,
		ProblemTopicID:   row.ProblemTopicID,
		ProblemTopicCode: row.ProblemTopicCode,
		ProblemTopicName: row.ProblemTopicName,
		ProblemTopics:    []ProblemTopicBrief{},
	}
	if row.ProblemTopicID != nil && row.ProblemTopicCode != nil && row.ProblemTopicName != nil {
		summary.ProblemTopics = []ProblemTopicBrief{{
			ID:   *row.ProblemTopicID,
			Code: *row.ProblemTopicCode,
			Name: *row.ProblemTopicName,
		}}
	}
	return summary
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

	items, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[OpenCmWorkOrderSummary])
	if err != nil {
		return nil, db.Translate(err)
	}
	return items, nil
}

func firstOpenCmConflictForTopics(ctx context.Context, q pgxQueryer, panelID uuid.UUID, check OpenCmDuplicateCheck) (*OpenCmWorkOrderSummary, error) {
	if len(check.TopicIDs) == 0 {
		return nil, nil
	}
	var exclude any
	if check.ExcludeWorkOrderID != nil {
		exclude = *check.ExcludeWorkOrderID
	}
	rows, err := q.Query(ctx, openCmConflictByTopicsSelect, panelID, check.TopicIDs, exclude)
	if err != nil {
		return nil, db.Translate(err)
	}
	defer rows.Close()
	item, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[openCmConflictRow])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, db.Translate(err)
	}
	summary := openCmConflictRowToSummary(item)
	return &summary, nil
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
	if filter.ProblemTopicID == nil {
		return nil
	}
	return EnsureNoOpenCmConflictForTopics(ctx, q, panelID, OpenCmDuplicateCheck{
		TopicIDs:           []uuid.UUID{*filter.ProblemTopicID},
		ExcludeWorkOrderID: filter.ExcludeWorkOrderID,
	})
}

// EnsureNoOpenCmConflictForTopics rejects when any topic is already covered by
// another open CM on the panel.
func EnsureNoOpenCmConflictForTopics(ctx context.Context, q pgxQueryer, panelID uuid.UUID, check OpenCmDuplicateCheck) error {
	conflict, err := firstOpenCmConflictForTopics(ctx, q, panelID, check)
	if err != nil {
		return err
	}
	if conflict != nil {
		return &OpenCmConflictError{Conflict: *conflict}
	}
	return nil
}
