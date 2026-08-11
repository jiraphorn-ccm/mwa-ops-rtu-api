-- name: UpsertPmGroundTest :one
INSERT INTO rtu.pm_ground_tests (
    pm_report_id, resistance_lg, resistance_ng, voltage_lg, voltage_ng,
    result, note, measured_by, measured_at, created_by, updated_by
)
VALUES (
    @pm_report_id::uuid,
    sqlc.narg('resistance_lg')::numeric,
    sqlc.narg('resistance_ng')::numeric,
    sqlc.narg('voltage_lg')::numeric,
    sqlc.narg('voltage_ng')::numeric,
    sqlc.narg('result')::varchar,
    sqlc.narg('note')::text,
    sqlc.narg('measured_by')::uuid,
    sqlc.narg('measured_at')::timestamptz,
    sqlc.narg('created_by')::uuid,
    sqlc.narg('updated_by')::uuid
)
ON CONFLICT (pm_report_id) DO UPDATE SET
    resistance_lg = EXCLUDED.resistance_lg,
    resistance_ng = EXCLUDED.resistance_ng,
    voltage_lg    = EXCLUDED.voltage_lg,
    voltage_ng    = EXCLUDED.voltage_ng,
    result        = EXCLUDED.result,
    note          = EXCLUDED.note,
    measured_by   = EXCLUDED.measured_by,
    measured_at   = EXCLUDED.measured_at,
    updated_by    = EXCLUDED.updated_by
RETURNING *;

-- name: GetPmGroundTestByReport :one
SELECT * FROM rtu.pm_ground_tests WHERE pm_report_id = @pm_report_id::uuid;

-- name: DeletePmGroundTestByReport :execrows
DELETE FROM rtu.pm_ground_tests WHERE pm_report_id = @pm_report_id::uuid;

-- name: PmGroundTestExists :one
SELECT EXISTS (SELECT 1 FROM rtu.pm_ground_tests WHERE id = @id::uuid) AS found;
