-- name: CreateEngineer :one
INSERT INTO rtu.engineers (full_name, license_no, position, active, created_by, updated_by)
VALUES (
    @full_name::varchar,
    sqlc.narg('license_no')::varchar,
    sqlc.narg('position')::varchar,
    COALESCE(sqlc.narg('active')::boolean, true),
    sqlc.narg('created_by')::uuid,
    sqlc.narg('updated_by')::uuid
)
RETURNING *;

-- name: GetEngineer :one
SELECT * FROM rtu.engineers WHERE id = @id::uuid;

-- name: UpdateEngineer :one
UPDATE rtu.engineers SET
    full_name  = CASE WHEN @full_name_do_update::boolean THEN @full_name::varchar ELSE full_name END,
    license_no = CASE WHEN @license_no_do_update::boolean THEN sqlc.narg('license_no')::varchar ELSE license_no END,
    position   = CASE WHEN @position_do_update::boolean THEN sqlc.narg('position')::varchar ELSE position END,
    active     = CASE WHEN @active_do_update::boolean THEN @active::boolean ELSE active END,
    updated_by = sqlc.narg('updated_by')::uuid
WHERE id = @id::uuid
RETURNING *;

-- name: SetEngineerActive :one
UPDATE rtu.engineers SET
    active     = @active::boolean,
    updated_by = sqlc.narg('updated_by')::uuid
WHERE id = @id::uuid
RETURNING *;

-- name: DeleteEngineer :execrows
DELETE FROM rtu.engineers WHERE id = @id::uuid;

-- name: EngineerExists :one
SELECT EXISTS (SELECT 1 FROM rtu.engineers WHERE id = @id::uuid) AS found;
