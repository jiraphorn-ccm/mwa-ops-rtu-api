-- name: CreateCalibration :one
INSERT INTO rtu.calibrations (
    panel_device_id, instrument_id, performed_by, performed_at, result, remark,
    work_order_id, pm_report_id, channel_type,
    eut_manufacturer, eut_model, eut_serial_no, eut_input_range,
    eut_accuracy_class, eut_power_supply, eut_output_range,
    result_type, result_other_text,
    created_by, updated_by
)
VALUES (
    @panel_device_id::uuid,
    @instrument_id::uuid,
    sqlc.narg('performed_by')::varchar,
    @performed_at::timestamptz,
    @result::varchar,
    sqlc.narg('remark')::text,
    sqlc.narg('work_order_id')::uuid,
    sqlc.narg('pm_report_id')::uuid,
    sqlc.narg('channel_type')::varchar,
    sqlc.narg('eut_manufacturer')::varchar,
    sqlc.narg('eut_model')::varchar,
    sqlc.narg('eut_serial_no')::varchar,
    sqlc.narg('eut_input_range')::varchar,
    sqlc.narg('eut_accuracy_class')::varchar,
    sqlc.narg('eut_power_supply')::varchar,
    sqlc.narg('eut_output_range')::varchar,
    sqlc.narg('result_type')::varchar,
    sqlc.narg('result_other_text')::varchar,
    sqlc.narg('created_by')::uuid,
    sqlc.narg('updated_by')::uuid
)
RETURNING *;

-- name: GetCalibration :one
SELECT * FROM rtu.calibrations WHERE id = @id::uuid;

-- name: UpdateCalibration :one
UPDATE rtu.calibrations SET
    panel_device_id    = CASE WHEN @panel_device_id_do_update::boolean THEN @panel_device_id::uuid ELSE panel_device_id END,
    instrument_id       = CASE WHEN @instrument_id_do_update::boolean THEN @instrument_id::uuid ELSE instrument_id END,
    performed_by        = CASE WHEN @performed_by_do_update::boolean THEN sqlc.narg('performed_by')::varchar ELSE performed_by END,
    performed_at        = CASE WHEN @performed_at_do_update::boolean THEN @performed_at::timestamptz ELSE performed_at END,
    result              = CASE WHEN @result_do_update::boolean THEN @result::varchar ELSE result END,
    remark              = CASE WHEN @remark_do_update::boolean THEN sqlc.narg('remark')::text ELSE remark END,
    work_order_id       = CASE WHEN @work_order_id_do_update::boolean THEN sqlc.narg('work_order_id')::uuid ELSE work_order_id END,
    pm_report_id        = CASE WHEN @pm_report_id_do_update::boolean THEN sqlc.narg('pm_report_id')::uuid ELSE pm_report_id END,
    channel_type        = CASE WHEN @channel_type_do_update::boolean THEN sqlc.narg('channel_type')::varchar ELSE channel_type END,
    eut_manufacturer    = CASE WHEN @eut_manufacturer_do_update::boolean THEN sqlc.narg('eut_manufacturer')::varchar ELSE eut_manufacturer END,
    eut_model           = CASE WHEN @eut_model_do_update::boolean THEN sqlc.narg('eut_model')::varchar ELSE eut_model END,
    eut_serial_no       = CASE WHEN @eut_serial_no_do_update::boolean THEN sqlc.narg('eut_serial_no')::varchar ELSE eut_serial_no END,
    eut_input_range     = CASE WHEN @eut_input_range_do_update::boolean THEN sqlc.narg('eut_input_range')::varchar ELSE eut_input_range END,
    eut_accuracy_class  = CASE WHEN @eut_accuracy_class_do_update::boolean THEN sqlc.narg('eut_accuracy_class')::varchar ELSE eut_accuracy_class END,
    eut_power_supply    = CASE WHEN @eut_power_supply_do_update::boolean THEN sqlc.narg('eut_power_supply')::varchar ELSE eut_power_supply END,
    eut_output_range    = CASE WHEN @eut_output_range_do_update::boolean THEN sqlc.narg('eut_output_range')::varchar ELSE eut_output_range END,
    result_type         = CASE WHEN @result_type_do_update::boolean THEN sqlc.narg('result_type')::varchar ELSE result_type END,
    result_other_text   = CASE WHEN @result_other_text_do_update::boolean THEN sqlc.narg('result_other_text')::varchar ELSE result_other_text END,
    updated_by          = sqlc.narg('updated_by')::uuid
WHERE id = @id::uuid
RETURNING *;

-- name: DeleteCalibration :execrows
DELETE FROM rtu.calibrations WHERE id = @id::uuid;

-- name: CalibrationExists :one
SELECT EXISTS (SELECT 1 FROM rtu.calibrations WHERE id = @id::uuid) AS found;

-- name: ListCalibrationsByPmReport :many
SELECT * FROM rtu.calibrations WHERE pm_report_id = @pm_report_id::uuid ORDER BY performed_at;

-- name: ListCalibrationsForPmReport :many
-- Includes calibrations linked directly to the report and those linked only
-- to the PM work order (pm_report_id NULL) for submit/detail consistency.
SELECT * FROM rtu.calibrations
WHERE pm_report_id = @pm_report_id::uuid
   OR (work_order_id = @work_order_id::uuid AND pm_report_id IS NULL)
ORDER BY performed_at;

-- name: GetLatestCalibrationForDevice :one
SELECT * FROM rtu.calibrations
WHERE panel_device_id = @panel_device_id::uuid
ORDER BY performed_at DESC
LIMIT 1;

-- name: CountCalibrationsByResult :many
SELECT result, count(*)::bigint AS total
FROM rtu.calibrations
WHERE (sqlc.narg('panel_device_id')::uuid IS NULL OR panel_device_id = sqlc.narg('panel_device_id')::uuid)
  AND (sqlc.narg('performed_from')::timestamptz IS NULL OR performed_at >= sqlc.narg('performed_from')::timestamptz)
  AND (sqlc.narg('performed_to')::timestamptz IS NULL OR performed_at <= sqlc.narg('performed_to')::timestamptz)
GROUP BY result;
