-- name: CreatePanelImage :one
INSERT INTO rtu.panel_images (
    panel_id, image_type, s3_bucket, s3_key,
    original_name, mime_type, file_size, caption, sort_order,
    created_by, updated_by
)
VALUES (
    @panel_id::uuid,
    @image_type::varchar,
    @s3_bucket::varchar,
    @s3_key::varchar,
    sqlc.narg('original_name')::varchar,
    @mime_type::varchar,
    @file_size::bigint,
    sqlc.narg('caption')::text,
    COALESCE(sqlc.narg('sort_order')::smallint, 0),
    sqlc.narg('created_by')::uuid,
    sqlc.narg('updated_by')::uuid
)
RETURNING *;

-- name: GetPanelImage :one
SELECT * FROM rtu.panel_images WHERE id = @id::uuid;

-- name: GetPanelImageForPanel :one
SELECT * FROM rtu.panel_images
WHERE id = @id::uuid AND panel_id = @panel_id::uuid;

-- name: UpdatePanelImage :one
UPDATE rtu.panel_images SET
    image_type = CASE WHEN @image_type_do_update::boolean THEN @image_type::varchar ELSE image_type END,
    caption    = CASE WHEN @caption_do_update::boolean THEN sqlc.narg('caption')::text ELSE caption END,
    sort_order = CASE WHEN @sort_order_do_update::boolean THEN @sort_order::smallint ELSE sort_order END,
    updated_by = sqlc.narg('updated_by')::uuid
WHERE id = @id::uuid AND panel_id = @panel_id::uuid
RETURNING *;

-- name: ReplacePanelImageFile :one
UPDATE rtu.panel_images SET
    s3_key        = @s3_key::varchar,
    mime_type     = @mime_type::varchar,
    file_size     = @file_size::bigint,
    original_name = sqlc.narg('original_name')::varchar,
    updated_by    = sqlc.narg('updated_by')::uuid
WHERE id = @id::uuid AND panel_id = @panel_id::uuid
RETURNING *;

-- name: DeletePanelImage :execrows
DELETE FROM rtu.panel_images WHERE id = @id::uuid AND panel_id = @panel_id::uuid;
