-- name: CreateCalibrationReading :one
INSERT INTO rtu.calibration_readings (
    calibration_id, sequence, item_label, parameter_key, value, unit,
    created_by, updated_by
)
VALUES (
    @calibration_id::uuid,
    @sequence::smallint,
    sqlc.narg('item_label')::varchar,
    @parameter_key::varchar,
    sqlc.narg('value')::numeric,
    sqlc.narg('unit')::varchar,
    sqlc.narg('created_by')::uuid,
    sqlc.narg('updated_by')::uuid
)
RETURNING *;

-- name: BulkCreateCalibrationReadings :copyfrom
INSERT INTO rtu.calibration_readings (
    calibration_id, sequence, item_label, parameter_key, value, unit, created_by
) VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: GetCalibrationReading :one
SELECT * FROM rtu.calibration_readings WHERE id = @id::uuid;

-- name: ListCalibrationReadings :many
SELECT * FROM rtu.calibration_readings
WHERE calibration_id = @calibration_id::uuid
ORDER BY sequence;

-- name: ListCalibrationReadingsForCalibrations :many
SELECT * FROM rtu.calibration_readings
WHERE calibration_id = ANY (@calibration_ids::uuid[])
ORDER BY calibration_id, sequence;

-- name: GetCalibrationReadingForCalibration :one
SELECT * FROM rtu.calibration_readings
WHERE id = @id::uuid AND calibration_id = @calibration_id::uuid;

-- name: UpdateCalibrationReading :one
UPDATE rtu.calibration_readings SET
    sequence      = CASE WHEN @sequence_do_update::boolean THEN @sequence::smallint ELSE sequence END,
    item_label    = CASE WHEN @item_label_do_update::boolean THEN sqlc.narg('item_label')::varchar ELSE item_label END,
    parameter_key = CASE WHEN @parameter_key_do_update::boolean THEN @parameter_key::varchar ELSE parameter_key END,
    value         = CASE WHEN @value_do_update::boolean THEN sqlc.narg('value')::numeric ELSE value END,
    unit          = CASE WHEN @unit_do_update::boolean THEN sqlc.narg('unit')::varchar ELSE unit END,
    updated_by    = sqlc.narg('updated_by')::uuid
WHERE id = @id::uuid
RETURNING *;

-- name: DeleteCalibrationReading :execrows
DELETE FROM rtu.calibration_readings WHERE id = @id::uuid;

-- name: DeleteCalibrationReadingsByCalibration :execrows
DELETE FROM rtu.calibration_readings WHERE calibration_id = @calibration_id::uuid;

-- name: NextCalibrationReadingSequence :one
SELECT (COALESCE(max(sequence), 0) + 1)::smallint AS next_sequence
FROM rtu.calibration_readings
WHERE calibration_id = @calibration_id::uuid;
