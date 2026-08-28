-- name: CreatePanelDevice :one
INSERT INTO rtu.panel_devices (
    panel_id, name, equipment_type, manufacturer, brand, model,
    serial_number, calibration_date, expire_date,
    tag_name, asset_code, firmware_version,
    communication_status, health_status,
    installed_at, last_seen_at, note, active, created_by, updated_by
)
VALUES (
    @panel_id::uuid,
    @name::varchar,
    sqlc.narg('equipment_type')::varchar,
    sqlc.narg('manufacturer')::varchar,
    sqlc.narg('brand')::varchar,
    sqlc.narg('model')::varchar,
    sqlc.narg('serial_number')::varchar,
    sqlc.narg('calibration_date')::date,
    sqlc.narg('expire_date')::date,
    sqlc.narg('tag_name')::varchar,
    sqlc.narg('asset_code')::varchar,
    sqlc.narg('firmware_version')::varchar,
    COALESCE(sqlc.narg('communication_status')::varchar, 'UNKNOWN'),
    COALESCE(sqlc.narg('health_status')::varchar, 'UNKNOWN'),
    sqlc.narg('installed_at')::date,
    sqlc.narg('last_seen_at')::timestamptz,
    sqlc.narg('note')::text,
    COALESCE(sqlc.narg('active')::boolean, true),
    sqlc.narg('created_by')::uuid,
    sqlc.narg('updated_by')::uuid
)
RETURNING *;

-- name: GetPanelDevice :one
SELECT * FROM rtu.panel_devices WHERE id = @id::uuid;

-- name: UpdatePanelDevice :one
UPDATE rtu.panel_devices SET
    panel_id             = CASE WHEN @panel_id_do_update::boolean THEN @panel_id::uuid ELSE panel_id END,
    name                 = CASE WHEN @name_do_update::boolean THEN @name::varchar ELSE name END,
    equipment_type       = CASE WHEN @equipment_type_do_update::boolean THEN sqlc.narg('equipment_type')::varchar ELSE equipment_type END,
    manufacturer         = CASE WHEN @manufacturer_do_update::boolean THEN sqlc.narg('manufacturer')::varchar ELSE manufacturer END,
    brand                = CASE WHEN @brand_do_update::boolean THEN sqlc.narg('brand')::varchar ELSE brand END,
    model                = CASE WHEN @model_do_update::boolean THEN sqlc.narg('model')::varchar ELSE model END,
    serial_number        = CASE WHEN @serial_number_do_update::boolean THEN sqlc.narg('serial_number')::varchar ELSE serial_number END,
    calibration_date     = CASE WHEN @calibration_date_do_update::boolean THEN sqlc.narg('calibration_date')::date ELSE calibration_date END,
    expire_date          = CASE WHEN @expire_date_do_update::boolean THEN sqlc.narg('expire_date')::date ELSE expire_date END,
    tag_name             = CASE WHEN @tag_name_do_update::boolean THEN sqlc.narg('tag_name')::varchar ELSE tag_name END,
    asset_code           = CASE WHEN @asset_code_do_update::boolean THEN sqlc.narg('asset_code')::varchar ELSE asset_code END,
    firmware_version     = CASE WHEN @firmware_version_do_update::boolean THEN sqlc.narg('firmware_version')::varchar ELSE firmware_version END,
    communication_status = CASE WHEN @communication_status_do_update::boolean THEN @communication_status::varchar ELSE communication_status END,
    health_status        = CASE WHEN @health_status_do_update::boolean THEN @health_status::varchar ELSE health_status END,
    installed_at         = CASE WHEN @installed_at_do_update::boolean THEN sqlc.narg('installed_at')::date ELSE installed_at END,
    last_seen_at         = CASE WHEN @last_seen_at_do_update::boolean THEN sqlc.narg('last_seen_at')::timestamptz ELSE last_seen_at END,
    note                 = CASE WHEN @note_do_update::boolean THEN sqlc.narg('note')::text ELSE note END,
    active               = CASE WHEN @active_do_update::boolean THEN @active::boolean ELSE active END,
    updated_by           = sqlc.narg('updated_by')::uuid
WHERE id = @id::uuid
RETURNING *;

-- name: UpdatePanelDeviceStatus :one
UPDATE rtu.panel_devices SET
    communication_status = CASE WHEN @communication_status_do_update::boolean THEN @communication_status::varchar ELSE communication_status END,
    health_status        = CASE WHEN @health_status_do_update::boolean THEN @health_status::varchar ELSE health_status END,
    last_seen_at         = COALESCE(sqlc.narg('last_seen_at')::timestamptz, now()),
    updated_by           = sqlc.narg('updated_by')::uuid
WHERE id = @id::uuid
RETURNING *;

-- name: SetPanelDeviceActive :one
UPDATE rtu.panel_devices SET
    active     = @active::boolean,
    updated_by = sqlc.narg('updated_by')::uuid
WHERE id = @id::uuid
RETURNING *;

-- name: DeletePanelDevice :execrows
DELETE FROM rtu.panel_devices WHERE id = @id::uuid;

-- name: PanelDeviceExists :one
SELECT EXISTS (SELECT 1 FROM rtu.panel_devices WHERE id = @id::uuid) AS found;

-- name: PanelDeviceIsActive :one
SELECT active FROM rtu.panel_devices WHERE id = @id::uuid;
