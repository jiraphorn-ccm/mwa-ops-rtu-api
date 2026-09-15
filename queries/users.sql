-- name: CreateUser :one
INSERT INTO rtu.users (
    employee_code, title, first_name, last_name, email, password_hash,
    position, active, created_by, updated_by
)
VALUES (
    @employee_code::varchar,
    sqlc.narg('title')::varchar,
    @first_name::varchar,
    @last_name::varchar,
    @email::varchar,
    @password_hash::text,
    sqlc.narg('position')::varchar,
    COALESCE(sqlc.narg('active')::boolean, true),
    sqlc.narg('created_by')::uuid,
    sqlc.narg('updated_by')::uuid
)
RETURNING *;

-- name: GetUser :one
SELECT * FROM rtu.users WHERE id = @id::uuid;

-- name: GetUserByLogin :one
SELECT * FROM rtu.users
WHERE lower(email) = lower(@login::varchar)
   OR employee_code = @login::varchar
LIMIT 1;

-- name: CountUsers :one
SELECT count(*)::bigint FROM rtu.users;

-- name: UpdateUser :one
UPDATE rtu.users SET
    employee_code = CASE WHEN @employee_code_do_update::boolean THEN @employee_code::varchar ELSE employee_code END,
    title         = CASE WHEN @title_do_update::boolean THEN sqlc.narg('title')::varchar ELSE title END,
    first_name    = CASE WHEN @first_name_do_update::boolean THEN @first_name::varchar ELSE first_name END,
    last_name     = CASE WHEN @last_name_do_update::boolean THEN @last_name::varchar ELSE last_name END,
    email         = CASE WHEN @email_do_update::boolean THEN @email::varchar ELSE email END,
    password_hash = CASE WHEN @password_hash_do_update::boolean THEN @password_hash::text ELSE password_hash END,
    position      = CASE WHEN @position_do_update::boolean THEN sqlc.narg('position')::varchar ELSE position END,
    active        = CASE WHEN @active_do_update::boolean THEN @active::boolean ELSE active END,
    updated_by    = sqlc.narg('updated_by')::uuid
WHERE id = @id::uuid
RETURNING *;

-- name: SetUserActive :one
UPDATE rtu.users SET
    active     = @active::boolean,
    updated_by = sqlc.narg('updated_by')::uuid
WHERE id = @id::uuid
RETURNING *;

-- name: SetUserPassword :one
UPDATE rtu.users SET
    password_hash = @password_hash::text,
    updated_by    = sqlc.narg('updated_by')::uuid
WHERE id = @id::uuid
RETURNING *;

-- name: TouchUserLastLogin :exec
UPDATE rtu.users SET last_login_at = now() WHERE id = @id::uuid;

-- name: DeleteUser :execrows
DELETE FROM rtu.users WHERE id = @id::uuid;
