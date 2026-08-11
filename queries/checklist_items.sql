-- name: CreateChecklistItem :one
INSERT INTO rtu.checklist_items (code, name, action_type, applicable_pm, sort_order, active, created_by, updated_by)
VALUES (
    @code::varchar,
    @name::varchar,
    @action_type::varchar,
    COALESCE(sqlc.narg('applicable_pm')::varchar, 'BOTH'),
    @sort_order::smallint,
    COALESCE(sqlc.narg('active')::boolean, true),
    sqlc.narg('created_by')::uuid,
    sqlc.narg('updated_by')::uuid
)
RETURNING *;

-- name: GetChecklistItem :one
SELECT * FROM rtu.checklist_items WHERE id = @id::uuid;

-- name: GetChecklistItemByCode :one
SELECT * FROM rtu.checklist_items WHERE code = @code::varchar;

-- name: UpdateChecklistItem :one
UPDATE rtu.checklist_items SET
    code          = CASE WHEN @code_do_update::boolean THEN @code::varchar ELSE code END,
    name          = CASE WHEN @name_do_update::boolean THEN @name::varchar ELSE name END,
    action_type   = CASE WHEN @action_type_do_update::boolean THEN @action_type::varchar ELSE action_type END,
    applicable_pm = CASE WHEN @applicable_pm_do_update::boolean THEN @applicable_pm::varchar ELSE applicable_pm END,
    sort_order    = CASE WHEN @sort_order_do_update::boolean THEN @sort_order::smallint ELSE sort_order END,
    active        = CASE WHEN @active_do_update::boolean THEN @active::boolean ELSE active END,
    updated_by    = sqlc.narg('updated_by')::uuid
WHERE id = @id::uuid
RETURNING *;

-- name: SetChecklistItemActive :one
UPDATE rtu.checklist_items SET
    active     = @active::boolean,
    updated_by = sqlc.narg('updated_by')::uuid
WHERE id = @id::uuid
RETURNING *;

-- name: DeleteChecklistItem :execrows
DELETE FROM rtu.checklist_items WHERE id = @id::uuid;

-- name: ListChecklistItems :many
SELECT * FROM rtu.checklist_items
WHERE (sqlc.narg('active_filter')::boolean IS NULL OR active = sqlc.narg('active_filter')::boolean)
ORDER BY sort_order ASC;
