-- name: UpsertPmPowerTest :one
INSERT INTO rtu.pm_power_tests (
    pm_report_id, instrument_id, tested_by, tested_at, created_by, updated_by
)
VALUES (
    @pm_report_id::uuid,
    sqlc.narg('instrument_id')::uuid,
    sqlc.narg('tested_by')::uuid,
    sqlc.narg('tested_at')::timestamptz,
    sqlc.narg('created_by')::uuid,
    sqlc.narg('updated_by')::uuid
)
ON CONFLICT (pm_report_id) DO UPDATE SET
    instrument_id = EXCLUDED.instrument_id,
    tested_by     = EXCLUDED.tested_by,
    tested_at     = EXCLUDED.tested_at,
    updated_by    = EXCLUDED.updated_by
RETURNING *;

-- name: GetPmPowerTestByReport :one
SELECT * FROM rtu.pm_power_tests WHERE pm_report_id = @pm_report_id::uuid;

-- name: DeletePmPowerTestByReport :execrows
DELETE FROM rtu.pm_power_tests WHERE pm_report_id = @pm_report_id::uuid;
