-- name: CreatePmReport :one
INSERT INTO rtu.pm_reports (
    work_order_id, work_order_round_id, panel_id, engineer_id,
    note, report_date, created_by, updated_by
)
VALUES (
    @work_order_id::uuid,
    @work_order_round_id::uuid,
    @panel_id::uuid,
    sqlc.narg('engineer_id')::uuid,
    sqlc.narg('note')::text,
    sqlc.narg('report_date')::timestamptz,
    sqlc.narg('created_by')::uuid,
    sqlc.narg('updated_by')::uuid
)
RETURNING *;

-- name: GetPmReport :one
SELECT * FROM rtu.pm_reports WHERE id = @id::uuid;

-- name: GetPmReportByRound :one
SELECT * FROM rtu.pm_reports WHERE work_order_round_id = @work_order_round_id::uuid;

-- name: UpdatePmReport :one
UPDATE rtu.pm_reports SET
    engineer_id  = CASE WHEN @engineer_id_do_update::boolean THEN sqlc.narg('engineer_id')::uuid ELSE engineer_id END,
    note         = CASE WHEN @note_do_update::boolean THEN sqlc.narg('note')::text ELSE note END,
    report_date  = CASE WHEN @report_date_do_update::boolean THEN sqlc.narg('report_date')::timestamptz ELSE report_date END,
    updated_by   = sqlc.narg('updated_by')::uuid
WHERE id = @id::uuid AND status = 'DRAFT'
RETURNING *;

-- name: SetPmReportSubmitted :one
UPDATE rtu.pm_reports SET
    status       = 'SUBMITTED',
    submitted_by = sqlc.narg('submitted_by')::uuid,
    submitted_at = @submitted_at::timestamptz,
    updated_by   = sqlc.narg('updated_by')::uuid
WHERE id = @id::uuid AND status = 'DRAFT'
RETURNING *;

-- name: DeletePmReport :execrows
DELETE FROM rtu.pm_reports WHERE id = @id::uuid AND status = 'DRAFT';

-- name: PmReportExists :one
SELECT EXISTS (SELECT 1 FROM rtu.pm_reports WHERE id = @id::uuid) AS found;
