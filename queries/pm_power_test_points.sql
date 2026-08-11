-- name: BulkCreatePmPowerTestPoints :copyfrom
INSERT INTO rtu.pm_power_test_points (
    pm_power_test_id, equipment_role, brand, model,
    input_accept_range, input_result_value, input_unit,
    output_accept_range, output_result_value, output_unit,
    result, corrective_action, created_by
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13);

-- name: ListPmPowerTestPointsByTest :many
SELECT * FROM rtu.pm_power_test_points
WHERE pm_power_test_id = @pm_power_test_id::uuid
ORDER BY equipment_role;

-- name: DeletePmPowerTestPointsByTest :execrows
DELETE FROM rtu.pm_power_test_points WHERE pm_power_test_id = @pm_power_test_id::uuid;

-- name: PmPowerTestPointExists :one
SELECT EXISTS (SELECT 1 FROM rtu.pm_power_test_points WHERE id = @id::uuid) AS found;
