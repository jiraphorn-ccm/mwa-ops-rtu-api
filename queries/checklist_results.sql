-- name: BulkCreateChecklistResults :copyfrom
INSERT INTO rtu.checklist_results (
    pm_report_id, checklist_item_id, panel_device_id, status, value, meter_no,
    note, checked_by, checked_at, created_by
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- name: ListChecklistResultsByReport :many
SELECT cr.*, ci.code AS item_code, ci.name AS item_name, ci.action_type AS item_action_type
FROM rtu.checklist_results cr
JOIN rtu.checklist_items ci ON ci.id = cr.checklist_item_id
WHERE cr.pm_report_id = @pm_report_id::uuid
ORDER BY ci.sort_order, cr.panel_device_id NULLS FIRST;

-- name: DeleteChecklistResultsByReport :execrows
DELETE FROM rtu.checklist_results WHERE pm_report_id = @pm_report_id::uuid;
