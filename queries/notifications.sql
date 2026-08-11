-- name: CreateNotification :one
INSERT INTO rtu.notifications (
    work_order_id, recipient_id, type, title, message, created_by, updated_by
)
VALUES (
    @work_order_id::uuid,
    @recipient_id::uuid,
    @type::varchar,
    sqlc.narg('title')::varchar,
    sqlc.narg('message')::text,
    sqlc.narg('created_by')::uuid,
    sqlc.narg('updated_by')::uuid
)
RETURNING *;

-- name: GetNotification :one
SELECT * FROM rtu.notifications WHERE id = @id::uuid;

-- name: MarkNotificationRead :one
UPDATE rtu.notifications SET
    is_read    = true,
    read_at    = now(),
    updated_by = sqlc.narg('updated_by')::uuid
WHERE id = @id::uuid AND recipient_id = @recipient_id::uuid
RETURNING *;

-- name: MarkAllNotificationsRead :execrows
UPDATE rtu.notifications SET
    is_read    = true,
    read_at    = now(),
    updated_by = sqlc.narg('updated_by')::uuid
WHERE recipient_id = @recipient_id::uuid AND is_read = false;

-- name: CountUnreadNotifications :one
SELECT count(*)::bigint FROM rtu.notifications
WHERE recipient_id = @recipient_id::uuid AND is_read = false;

-- name: DeleteNotification :execrows
DELETE FROM rtu.notifications WHERE id = @id::uuid AND recipient_id = @recipient_id::uuid;
