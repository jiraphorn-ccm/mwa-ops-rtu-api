-- name: CreateCalibrationInstrument :one
INSERT INTO rtu.calibration_instruments (
    name, equipment_type, manufacturer, brand, model, serial_number,
    calibration_date, expire_date, active,
    created_by, updated_by
)
VALUES (
    @name::varchar,
    sqlc.narg('equipment_type')::varchar,
    sqlc.narg('manufacturer')::varchar,
    sqlc.narg('brand')::varchar,
    sqlc.narg('model')::varchar,
    sqlc.narg('serial_number')::varchar,
    sqlc.narg('calibration_date')::date,
    sqlc.narg('expire_date')::date,
    COALESCE(sqlc.narg('active')::boolean, true),
    sqlc.narg('created_by')::uuid,
    sqlc.narg('updated_by')::uuid
)
RETURNING *;

-- name: GetCalibrationInstrument :one
SELECT * FROM rtu.calibration_instruments WHERE id = @id::uuid;

-- name: UpdateCalibrationInstrument :one
UPDATE rtu.calibration_instruments SET
    name             = CASE WHEN @name_do_update::boolean THEN @name::varchar ELSE name END,
    equipment_type   = CASE WHEN @equipment_type_do_update::boolean THEN sqlc.narg('equipment_type')::varchar ELSE equipment_type END,
    manufacturer     = CASE WHEN @manufacturer_do_update::boolean THEN sqlc.narg('manufacturer')::varchar ELSE manufacturer END,
    brand            = CASE WHEN @brand_do_update::boolean THEN sqlc.narg('brand')::varchar ELSE brand END,
    model            = CASE WHEN @model_do_update::boolean THEN sqlc.narg('model')::varchar ELSE model END,
    serial_number    = CASE WHEN @serial_number_do_update::boolean THEN sqlc.narg('serial_number')::varchar ELSE serial_number END,
    calibration_date = CASE WHEN @calibration_date_do_update::boolean THEN sqlc.narg('calibration_date')::date ELSE calibration_date END,
    expire_date      = CASE WHEN @expire_date_do_update::boolean THEN sqlc.narg('expire_date')::date ELSE expire_date END,
    active           = CASE WHEN @active_do_update::boolean THEN @active::boolean ELSE active END,
    updated_by       = sqlc.narg('updated_by')::uuid
WHERE id = @id::uuid
RETURNING *;

-- name: SetCalibrationInstrumentActive :one
UPDATE rtu.calibration_instruments SET
    active     = @active::boolean,
    updated_by = sqlc.narg('updated_by')::uuid
WHERE id = @id::uuid
RETURNING *;

-- name: DeleteCalibrationInstrument :execrows
DELETE FROM rtu.calibration_instruments WHERE id = @id::uuid;

-- name: CalibrationInstrumentExists :one
SELECT EXISTS (SELECT 1 FROM rtu.calibration_instruments WHERE id = @id::uuid) AS found;

-- name: GetCalibrationInstrumentUsability :one
SELECT active, expire_date FROM rtu.calibration_instruments WHERE id = @id::uuid;
