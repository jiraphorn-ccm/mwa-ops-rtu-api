-- name: CreateWorkOrderRound :one
INSERT INTO rtu.work_order_rounds (
    work_order_id, round_no, assigned_to, assigned_by, assigned_at, created_by, updated_by
)
VALUES (
    @work_order_id::uuid,
    @round_no::smallint,
    @assigned_to::uuid,
    @assigned_by::uuid,
    @assigned_at::timestamptz,
    sqlc.narg('created_by')::uuid,
    sqlc.narg('updated_by')::uuid
)
RETURNING *;

-- name: GetWorkOrderRound :one
SELECT * FROM rtu.work_order_rounds WHERE id = @id::uuid;

-- name: ListWorkOrderRoundsByWorkOrder :many
SELECT * FROM rtu.work_order_rounds
WHERE work_order_id = @work_order_id::uuid
ORDER BY round_no ASC;

-- name: NextWorkOrderRoundNo :one
SELECT (COALESCE(max(round_no), 0) + 1)::smallint AS next_round_no
FROM rtu.work_order_rounds
WHERE work_order_id = @work_order_id::uuid;

-- name: UpdateWorkOrderRoundAssignment :one
-- Reassigns the current round in place. Only valid before check-in — once a
-- round has started, a rejection must open a new round instead (see
-- CreateWorkOrderRound + NextWorkOrderRoundNo).
UPDATE rtu.work_order_rounds SET
    assigned_to = @assigned_to::uuid,
    assigned_by = @assigned_by::uuid,
    assigned_at = @assigned_at::timestamptz,
    updated_by  = sqlc.narg('updated_by')::uuid
WHERE id = @id::uuid AND check_in_at IS NULL
RETURNING *;

-- name: CheckInWorkOrderRound :one
UPDATE rtu.work_order_rounds SET
    check_in_at  = @check_in_at::timestamptz,
    check_in_lat = sqlc.narg('check_in_lat')::numeric,
    check_in_lng = sqlc.narg('check_in_lng')::numeric,
    updated_by   = sqlc.narg('updated_by')::uuid
WHERE id = @id::uuid AND check_in_at IS NULL
RETURNING *;

-- name: CheckOutWorkOrderRound :one
UPDATE rtu.work_order_rounds SET
    check_out_at  = @check_out_at::timestamptz,
    check_out_lat = sqlc.narg('check_out_lat')::numeric,
    check_out_lng = sqlc.narg('check_out_lng')::numeric,
    updated_by    = sqlc.narg('updated_by')::uuid
WHERE id = @id::uuid AND check_in_at IS NOT NULL AND check_out_at IS NULL
RETURNING *;

-- name: SetWorkOrderRoundSubmitted :one
UPDATE rtu.work_order_rounds SET
    submitted_at = COALESCE(sqlc.narg('submitted_at')::timestamptz, now()),
    updated_by   = sqlc.narg('updated_by')::uuid
WHERE id = @id::uuid
RETURNING *;
