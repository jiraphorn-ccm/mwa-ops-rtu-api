-- name: CreateAttachment :one
INSERT INTO rtu.attachments (
    entity_type, entity_id, s3_bucket, s3_key, original_name, mime_type,
    file_size, caption, created_by
)
VALUES (
    @entity_type::varchar,
    @entity_id::uuid,
    @s3_bucket::varchar,
    @s3_key::varchar,
    sqlc.narg('original_name')::varchar,
    @mime_type::varchar,
    @file_size::bigint,
    sqlc.narg('caption')::text,
    @created_by::uuid
)
RETURNING *;

-- name: GetAttachment :one
SELECT * FROM rtu.attachments WHERE id = @id::uuid;

-- name: ListAttachmentsByEntity :many
SELECT * FROM rtu.attachments
WHERE entity_type = @entity_type::varchar AND entity_id = @entity_id::uuid
ORDER BY created_at;

-- name: UpdateAttachmentCaption :one
UPDATE rtu.attachments SET
    caption    = sqlc.narg('caption')::text,
    updated_by = sqlc.narg('updated_by')::uuid
WHERE id = @id::uuid
RETURNING *;

-- name: DeleteAttachment :execrows
DELETE FROM rtu.attachments WHERE id = @id::uuid;
