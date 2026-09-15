-- name: CreateRefreshToken :one
INSERT INTO rtu.refresh_tokens (
    user_id, token_hash, ip_address, user_agent, expires_at
)
VALUES (
    @user_id::uuid,
    @token_hash::varchar,
    sqlc.narg('ip_address')::varchar,
    sqlc.narg('user_agent')::text,
    @expires_at::timestamptz
)
RETURNING *;

-- name: GetRefreshTokenByHash :one
SELECT
    rt.id, rt.user_id, rt.token_hash, rt.ip_address, rt.user_agent,
    rt.revoked_at, rt.expires_at, rt.created_at, rt.updated_at,
    u.active AS user_active
FROM rtu.refresh_tokens rt
INNER JOIN rtu.users u ON u.id = rt.user_id
WHERE rt.token_hash = @token_hash::varchar;

-- name: TouchRefreshTokenMeta :exec
UPDATE rtu.refresh_tokens SET
    ip_address = sqlc.narg('ip_address')::varchar,
    user_agent = sqlc.narg('user_agent')::text
WHERE id = @id::uuid;

-- name: RevokeRefreshToken :exec
UPDATE rtu.refresh_tokens SET revoked_at = now()
WHERE id = @id::uuid AND revoked_at IS NULL;

-- name: RevokeRefreshTokensForUser :execrows
UPDATE rtu.refresh_tokens SET revoked_at = now()
WHERE user_id = @user_id::uuid AND revoked_at IS NULL;
