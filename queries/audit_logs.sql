-- name: CreateAuditLog :one
INSERT INTO rtu.audit_logs (
    user_id, action, method, path, resource, resource_id,
    status_code, ip_address, user_agent, request_id
)
VALUES (
    sqlc.narg('user_id')::uuid,
    @action::varchar,
    @method::varchar,
    @path::text,
    sqlc.narg('resource')::varchar,
    sqlc.narg('resource_id')::uuid,
    sqlc.narg('status_code')::integer,
    sqlc.narg('ip_address')::varchar,
    sqlc.narg('user_agent')::text,
    sqlc.narg('request_id')::varchar
)
RETURNING *;
