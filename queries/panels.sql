-- name: CreatePanel :one
INSERT INTO rtu.panels (code, location, latitude, longitude, install_date, active, created_by, updated_by)
VALUES (
    @code::varchar,
    sqlc.narg('location')::text,
    sqlc.narg('latitude')::numeric,
    sqlc.narg('longitude')::numeric,
    sqlc.narg('install_date')::date,
    COALESCE(sqlc.narg('active')::boolean, true),
    sqlc.narg('created_by')::uuid,
    sqlc.narg('updated_by')::uuid
)
RETURNING *;

-- name: GetPanel :one
SELECT * FROM rtu.panels WHERE id = @id::uuid;

-- name: GetPanelByCode :one
SELECT * FROM rtu.panels WHERE code = @code::varchar;

-- name: UpdatePanel :one
UPDATE rtu.panels SET
    code         = CASE WHEN @code_do_update::boolean THEN @code::varchar ELSE code END,
    location     = CASE WHEN @location_do_update::boolean THEN sqlc.narg('location')::text ELSE location END,
    latitude     = CASE WHEN @latitude_do_update::boolean THEN sqlc.narg('latitude')::numeric ELSE latitude END,
    longitude    = CASE WHEN @longitude_do_update::boolean THEN sqlc.narg('longitude')::numeric ELSE longitude END,
    install_date = CASE WHEN @install_date_do_update::boolean THEN sqlc.narg('install_date')::date ELSE install_date END,
    active       = CASE WHEN @active_do_update::boolean THEN @active::boolean ELSE active END,
    updated_by   = sqlc.narg('updated_by')::uuid
WHERE id = @id::uuid
RETURNING *;

-- name: UpdatePanelPmDates :one
UPDATE rtu.panels SET
    last_pm_date = @last_pm_date::date,
    next_pm_date = @next_pm_date::date,
    updated_by   = sqlc.narg('updated_by')::uuid
WHERE id = @id::uuid
RETURNING *;

-- name: SetPanelActive :one
UPDATE rtu.panels SET
    active     = @active::boolean,
    updated_by = sqlc.narg('updated_by')::uuid
WHERE id = @id::uuid
RETURNING *;

-- name: DeletePanel :execrows
DELETE FROM rtu.panels WHERE id = @id::uuid;

-- name: PanelExists :one
SELECT EXISTS (SELECT 1 FROM rtu.panels WHERE id = @id::uuid) AS found;

-- name: PanelIsActive :one
SELECT active FROM rtu.panels WHERE id = @id::uuid;
