-- name: CreateWoApproval :one
INSERT INTO rtu.wo_approvals (
    work_order_id, work_order_round_id, reviewer_id, decision, repair_date,
    new_work_order_id, note, reviewed_at, created_by, updated_by
)
VALUES (
    @work_order_id::uuid,
    @work_order_round_id::uuid,
    @reviewer_id::uuid,
    @decision::varchar,
    sqlc.narg('repair_date')::date,
    sqlc.narg('new_work_order_id')::uuid,
    sqlc.narg('note')::text,
    COALESCE(sqlc.narg('reviewed_at')::timestamptz, now()),
    sqlc.narg('created_by')::uuid,
    sqlc.narg('updated_by')::uuid
)
RETURNING *;

-- name: GetWoApproval :one
SELECT * FROM rtu.wo_approvals WHERE id = @id::uuid;

-- name: GetWoApprovalByRound :one
SELECT * FROM rtu.wo_approvals WHERE work_order_round_id = @work_order_round_id::uuid;

-- name: ListWoApprovalsByWorkOrder :many
SELECT * FROM rtu.wo_approvals
WHERE work_order_id = @work_order_id::uuid
ORDER BY reviewed_at ASC;
