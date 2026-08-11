-- name: CreateWorkOrder :one
INSERT INTO rtu.work_orders (
    work_order_no, work_order_type, pm_schedule_type, panel_id, panel_device_id,
    title, description, priority, source, requested_by, related_work_order_id,
    planned_date, due_date, created_by, updated_by
)
VALUES (
    @work_order_no::varchar,
    @work_order_type::varchar,
    sqlc.narg('pm_schedule_type')::varchar,
    @panel_id::uuid,
    sqlc.narg('panel_device_id')::uuid,
    sqlc.narg('title')::varchar,
    sqlc.narg('description')::text,
    COALESCE(sqlc.narg('priority')::varchar, 'MEDIUM'),
    COALESCE(sqlc.narg('source')::varchar, 'WORKFLOW'),
    @requested_by::uuid,
    sqlc.narg('related_work_order_id')::uuid,
    sqlc.narg('planned_date')::date,
    sqlc.narg('due_date')::date,
    sqlc.narg('created_by')::uuid,
    sqlc.narg('updated_by')::uuid
)
RETURNING *;

-- name: GetWorkOrder :one
SELECT * FROM rtu.work_orders WHERE id = @id::uuid;

-- name: WorkOrderExists :one
SELECT EXISTS (SELECT 1 FROM rtu.work_orders WHERE id = @id::uuid) AS found;

-- name: CountWorkOrdersByPanelAndType :one
-- Used to generate the human-readable work_order_no sequence, e.g. PM-U120-4.
SELECT count(*)::bigint AS total
FROM rtu.work_orders
WHERE panel_id = @panel_id::uuid AND work_order_type = @work_order_type::varchar;

-- name: CountOpenWorkOrdersForPanel :one
-- Business rule: before opening a new CM work order, check whether an open
-- (status <> COMPLETED/CANCELLED) one already exists for the panel to reuse.
SELECT id FROM rtu.work_orders
WHERE panel_id = @panel_id::uuid
  AND work_order_type = @work_order_type::varchar
  AND status NOT IN ('COMPLETED', 'CANCELLED')
ORDER BY created_at DESC
LIMIT 1;

-- name: FindReusableCmWorkOrderForPanel :one
-- Reuse only a PENDING CM whose current round has no report yet — avoids
-- hijacking an in-progress CM or overwriting an existing round report.
SELECT wo.id
FROM rtu.work_orders wo
WHERE wo.panel_id = @panel_id::uuid
  AND wo.work_order_type = 'CM'
  AND wo.status = 'PENDING'
  AND wo.current_round_id IS NOT NULL
  AND NOT EXISTS (
    SELECT 1 FROM rtu.cm_reports cr
    WHERE cr.work_order_round_id = wo.current_round_id
  )
ORDER BY wo.created_at DESC
LIMIT 1;

-- name: UpdateWorkOrder :one
UPDATE rtu.work_orders SET
    panel_device_id = CASE WHEN @panel_device_id_do_update::boolean THEN sqlc.narg('panel_device_id')::uuid ELSE panel_device_id END,
    title           = CASE WHEN @title_do_update::boolean THEN sqlc.narg('title')::varchar ELSE title END,
    description     = CASE WHEN @description_do_update::boolean THEN sqlc.narg('description')::text ELSE description END,
    priority        = CASE WHEN @priority_do_update::boolean THEN @priority::varchar ELSE priority END,
    planned_date    = CASE WHEN @planned_date_do_update::boolean THEN sqlc.narg('planned_date')::date ELSE planned_date END,
    due_date        = CASE WHEN @due_date_do_update::boolean THEN sqlc.narg('due_date')::date ELSE due_date END,
    updated_by      = sqlc.narg('updated_by')::uuid
WHERE id = @id::uuid
RETURNING *;

-- name: SetWorkOrderCurrentRound :one
UPDATE rtu.work_orders SET
    current_round_id = @current_round_id::uuid,
    updated_by        = sqlc.narg('updated_by')::uuid
WHERE id = @id::uuid
RETURNING *;

-- name: UpdateWorkOrderStatus :one
UPDATE rtu.work_orders SET
    status     = @status::varchar,
    closed_at  = CASE WHEN @closed_at_do_update::boolean THEN sqlc.narg('closed_at')::timestamptz ELSE closed_at END,
    updated_by = sqlc.narg('updated_by')::uuid
WHERE id = @id::uuid
RETURNING *;

-- name: SetWorkOrderActive :one
UPDATE rtu.work_orders SET
    active     = @active::boolean,
    status     = CASE WHEN @active::boolean THEN status ELSE 'CANCELLED' END,
    updated_by = sqlc.narg('updated_by')::uuid
WHERE id = @id::uuid
RETURNING *;
