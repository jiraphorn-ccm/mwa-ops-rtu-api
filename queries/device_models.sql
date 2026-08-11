-- name: CreateDeviceModel :one
INSERT INTO rtu.device_models (code, name, manufacturer, model, description, active, created_by, updated_by)
VALUES (
    @code::varchar,
    @name::varchar,
    sqlc.narg('manufacturer')::varchar,
    sqlc.narg('model')::varchar,
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
    code         = CASE WHEN @code_do_update::boolean THEN @code::varchar ELSE code END,
    name         = CASE WHEN @name_do_update::boolean THEN @name::varchar ELSE name END,
    manufacturer = CASE WHEN @manufacturer_do_update::boolean THEN sqlc.narg('manufacturer')::varchar ELSE manufacturer END,
    model        = CASE WHEN @model_do_update::boolean THEN sqlc.narg('model')::varchar ELSE model END,
    description  = CASE WHEN @description_do_update::boolean THEN sqlc.narg('description')::text ELSE description END,
    active       = CASE WHEN @active_do_update::boolean THEN @active::boolean ELSE active END,
    updated_by   = sqlc.narg('updated_by')::uuid
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
