-- name: CreateCmReport :one
INSERT INTO rtu.cm_reports (
    work_order_id, work_order_round_id, pm_report_id, panel_id, panel_device_id,
    reported_by, problem_topic_id, tag_code, error_logs, problem_detail, root_cause, reference_info,
    corrective_action, recommendation, pending_reason, repaired_by,
    reported_at, started_at, ended_at, created_by, updated_by
)
VALUES (
    sqlc.narg('work_order_id')::uuid,
    sqlc.narg('work_order_round_id')::uuid,
    sqlc.narg('pm_report_id')::uuid,
    @panel_id::uuid,
    sqlc.narg('panel_device_id')::uuid,
    @reported_by::uuid,
    sqlc.narg('problem_topic_id')::uuid,
    sqlc.narg('tag_code')::varchar,
    sqlc.narg('error_logs')::text,
    sqlc.narg('problem_detail')::text,
    sqlc.narg('root_cause')::text,
    sqlc.narg('reference_info')::text,
    sqlc.narg('corrective_action')::text,
    sqlc.narg('recommendation')::text,
    sqlc.narg('pending_reason')::text,
    sqlc.narg('repaired_by')::uuid,
    sqlc.narg('reported_at')::timestamptz,
    sqlc.narg('started_at')::timestamptz,
    sqlc.narg('ended_at')::timestamptz,
    sqlc.narg('created_by')::uuid,
    sqlc.narg('updated_by')::uuid
)
RETURNING *;

-- name: GetCmReport :one
SELECT * FROM rtu.cm_reports WHERE id = @id::uuid;

-- name: GetCmReportByRound :one
SELECT * FROM rtu.cm_reports WHERE work_order_round_id = @work_order_round_id::uuid;

-- name: ListCmReportsByPmReport :many
SELECT * FROM rtu.cm_reports WHERE pm_report_id = @pm_report_id::uuid ORDER BY created_at;

-- name: UpdateCmReport :one
UPDATE rtu.cm_reports SET
    pm_report_id      = CASE WHEN @pm_report_id_do_update::boolean THEN sqlc.narg('pm_report_id')::uuid ELSE pm_report_id END,
    panel_device_id   = CASE WHEN @panel_device_id_do_update::boolean THEN sqlc.narg('panel_device_id')::uuid ELSE panel_device_id END,
    problem_topic_id  = CASE WHEN @problem_topic_id_do_update::boolean THEN sqlc.narg('problem_topic_id')::uuid ELSE problem_topic_id END,
    tag_code          = CASE WHEN @tag_code_do_update::boolean THEN sqlc.narg('tag_code')::varchar ELSE tag_code END,
    error_logs        = CASE WHEN @error_logs_do_update::boolean THEN sqlc.narg('error_logs')::text ELSE error_logs END,
    problem_detail    = CASE WHEN @problem_detail_do_update::boolean THEN sqlc.narg('problem_detail')::text ELSE problem_detail END,
    root_cause        = CASE WHEN @root_cause_do_update::boolean THEN sqlc.narg('root_cause')::text ELSE root_cause END,
    reference_info    = CASE WHEN @reference_info_do_update::boolean THEN sqlc.narg('reference_info')::text ELSE reference_info END,
    corrective_action = CASE WHEN @corrective_action_do_update::boolean THEN sqlc.narg('corrective_action')::text ELSE corrective_action END,
    recommendation    = CASE WHEN @recommendation_do_update::boolean THEN sqlc.narg('recommendation')::text ELSE recommendation END,
    pending_reason    = CASE WHEN @pending_reason_do_update::boolean THEN sqlc.narg('pending_reason')::text ELSE pending_reason END,
    repaired_by       = CASE WHEN @repaired_by_do_update::boolean THEN sqlc.narg('repaired_by')::uuid ELSE repaired_by END,
    reported_at       = CASE WHEN @reported_at_do_update::boolean THEN sqlc.narg('reported_at')::timestamptz ELSE reported_at END,
    started_at        = CASE WHEN @started_at_do_update::boolean THEN sqlc.narg('started_at')::timestamptz ELSE started_at END,
    ended_at          = CASE WHEN @ended_at_do_update::boolean THEN sqlc.narg('ended_at')::timestamptz ELSE ended_at END,
    updated_by        = sqlc.narg('updated_by')::uuid
WHERE id = @id::uuid
RETURNING *;

-- name: DeleteCmReport :execrows
DELETE FROM rtu.cm_reports WHERE id = @id::uuid;

-- name: CmReportExists :one
SELECT EXISTS (SELECT 1 FROM rtu.cm_reports WHERE id = @id::uuid) AS found;
