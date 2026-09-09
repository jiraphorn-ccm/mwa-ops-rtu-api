-- name: CreateDeviceModel :one
INSERT INTO rtu.device_models (
    code, name, equipment_type, manufacturer, brand, model,
    serial_number, expire_date, input_range, accuracy_class, power_supply, output_range,
    description, active, created_by, updated_by
)
VALUES (
    @code::varchar,
    @name::varchar,
    sqlc.narg('equipment_type')::varchar,
    sqlc.narg('manufacturer')::varchar,
    sqlc.narg('brand')::varchar,
    sqlc.narg('model')::varchar,
    sqlc.narg('serial_number')::varchar,
    sqlc.narg('expire_date')::date,
    sqlc.narg('input_range')::varchar,
    sqlc.narg('accuracy_class')::varchar,
    sqlc.narg('power_supply')::varchar,
    sqlc.narg('output_range')::varchar,
    sqlc.narg('description')::text,
    COALESCE(sqlc.narg('active')::boolean, true),
    sqlc.narg('created_by')::uuid,
    sqlc.narg('updated_by')::uuid
)
RETURNING *;

-- name: GetDeviceModel :one
SELECT * FROM rtu.device_models WHERE id = @id::uuid;

-- name: GetDeviceModelByCode :one
SELECT * FROM rtu.device_models WHERE code = @code::varchar;

-- name: UpdateDeviceModel :one
UPDATE rtu.device_models SET
    code            = CASE WHEN @code_do_update::boolean THEN @code::varchar ELSE code END,
    name            = CASE WHEN @name_do_update::boolean THEN @name::varchar ELSE name END,
    equipment_type  = CASE WHEN @equipment_type_do_update::boolean THEN sqlc.narg('equipment_type')::varchar ELSE equipment_type END,
    manufacturer    = CASE WHEN @manufacturer_do_update::boolean THEN sqlc.narg('manufacturer')::varchar ELSE manufacturer END,
    brand           = CASE WHEN @brand_do_update::boolean THEN sqlc.narg('brand')::varchar ELSE brand END,
    model           = CASE WHEN @model_do_update::boolean THEN sqlc.narg('model')::varchar ELSE model END,
    serial_number   = CASE WHEN @serial_number_do_update::boolean THEN sqlc.narg('serial_number')::varchar ELSE serial_number END,
    expire_date     = CASE WHEN @expire_date_do_update::boolean THEN sqlc.narg('expire_date')::date ELSE expire_date END,
    input_range     = CASE WHEN @input_range_do_update::boolean THEN sqlc.narg('input_range')::varchar ELSE input_range END,
    accuracy_class  = CASE WHEN @accuracy_class_do_update::boolean THEN sqlc.narg('accuracy_class')::varchar ELSE accuracy_class END,
    power_supply    = CASE WHEN @power_supply_do_update::boolean THEN sqlc.narg('power_supply')::varchar ELSE power_supply END,
    output_range    = CASE WHEN @output_range_do_update::boolean THEN sqlc.narg('output_range')::varchar ELSE output_range END,
    description     = CASE WHEN @description_do_update::boolean THEN sqlc.narg('description')::text ELSE description END,
    active          = CASE WHEN @active_do_update::boolean THEN @active::boolean ELSE active END,
    updated_by      = sqlc.narg('updated_by')::uuid
WHERE id = @id::uuid
RETURNING *;

-- name: SetDeviceModelActive :one
UPDATE rtu.device_models SET
    active     = @active::boolean,
    updated_by = sqlc.narg('updated_by')::uuid
WHERE id = @id::uuid
RETURNING *;

-- name: DeleteDeviceModel :execrows
DELETE FROM rtu.device_models WHERE id = @id::uuid;

-- name: DeviceModelExists :one
SELECT EXISTS (SELECT 1 FROM rtu.device_models WHERE id = @id::uuid) AS found;

-- name: DeviceModelIsActive :one
SELECT active FROM rtu.device_models WHERE id = @id::uuid;
