-- name: CreateWorkOrderActivityLog :one
INSERT INTO rtu.work_order_activity_logs (
    work_order_id, work_order_round_id, action, from_status, to_status,
    from_assignee, to_assignee, note, actor_id
)
VALUES (
    @work_order_id::uuid,
    sqlc.narg('work_order_round_id')::uuid,
    @action::varchar,
    sqlc.narg('from_status')::varchar,
    sqlc.narg('to_status')::varchar,
    sqlc.narg('from_assignee')::uuid,
    sqlc.narg('to_assignee')::uuid,
    sqlc.narg('note')::text,
    @actor_id::uuid
)
RETURNING *;

-- name: ListWorkOrderActivityLogs :many
SELECT * FROM rtu.work_order_activity_logs
WHERE work_order_id = @work_order_id::uuid
ORDER BY created_at ASC;
