-- name: CreateProblemTopic :one
INSERT INTO rtu.problem_topics (code, name, sort_order, active, created_by, updated_by)
VALUES (
    @code::varchar,
    @name::varchar,
    @sort_order::smallint,
    COALESCE(sqlc.narg('active')::boolean, true),
    sqlc.narg('created_by')::uuid,
    sqlc.narg('updated_by')::uuid
)
RETURNING *;

-- name: GetProblemTopic :one
SELECT * FROM rtu.problem_topics WHERE id = @id::uuid;

-- name: GetProblemTopicByCode :one
SELECT * FROM rtu.problem_topics WHERE code = @code::varchar;

-- name: UpdateProblemTopic :one
UPDATE rtu.problem_topics SET
    code       = CASE WHEN @code_do_update::boolean THEN @code::varchar ELSE code END,
    name       = CASE WHEN @name_do_update::boolean THEN @name::varchar ELSE name END,
    sort_order = CASE WHEN @sort_order_do_update::boolean THEN @sort_order::smallint ELSE sort_order END,
    active     = CASE WHEN @active_do_update::boolean THEN @active::boolean ELSE active END,
    updated_by = sqlc.narg('updated_by')::uuid
WHERE id = @id::uuid
RETURNING *;

-- name: SetProblemTopicActive :one
UPDATE rtu.problem_topics SET
    active     = @active::boolean,
    updated_by = sqlc.narg('updated_by')::uuid
WHERE id = @id::uuid
RETURNING *;

-- name: DeleteProblemTopic :execrows
DELETE FROM rtu.problem_topics WHERE id = @id::uuid;

-- name: ListProblemTopics :many
SELECT * FROM rtu.problem_topics
WHERE (sqlc.narg('active_filter')::boolean IS NULL OR active = sqlc.narg('active_filter')::boolean)
ORDER BY sort_order ASC;

-- name: GetProblemTopicUsability :one
SELECT active FROM rtu.problem_topics WHERE id = @id::uuid;

-- name: ProblemTopicExists :one
SELECT EXISTS (SELECT 1 FROM rtu.problem_topics WHERE id = @id::uuid) AS found;
