-- name: CreateAuthSession :one
INSERT INTO auth_sessions (
  user_id,
  client_name,
  user_agent,
  refresh_token_hash,
  refresh_token_family_id,
  last_used_at,
  expires_at,
  idle_expires_at
) VALUES (
  sqlc.arg(user_id),
  sqlc.arg(client_name),
  sqlc.arg(user_agent),
  sqlc.arg(refresh_token_hash),
  sqlc.arg(refresh_token_family_id),
  sqlc.narg(last_used_at),
  sqlc.arg(expires_at),
  sqlc.arg(idle_expires_at)
)
RETURNING *;

-- name: GetAuthSessionByID :one
SELECT *
FROM auth_sessions
WHERE id = sqlc.arg(id)
LIMIT 1;

-- name: GetAuthSessionByRefreshTokenHash :one
SELECT *
FROM auth_sessions
WHERE refresh_token_hash = sqlc.arg(refresh_token_hash)
LIMIT 1;

-- name: ListAuthSessions :many
SELECT *
FROM auth_sessions
ORDER BY created_at DESC, id DESC;

-- name: ListAuthSessionsByUserID :many
SELECT *
FROM auth_sessions
WHERE user_id = sqlc.arg(user_id)
ORDER BY created_at DESC, id DESC;

-- name: RevokeAuthSession :one
UPDATE auth_sessions
SET
  revoked_at = NOW(),
  revoke_reason = sqlc.arg(revoke_reason),
  updated_at = NOW()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: RevokeOtherAuthSessionsForUser :execrows
UPDATE auth_sessions
SET
  revoked_at = NOW(),
  revoke_reason = sqlc.arg(revoke_reason),
  updated_at = NOW()
WHERE user_id = sqlc.arg(user_id)
  AND revoked_at IS NULL
  AND id <> sqlc.arg(id);

-- name: RevokeAuthSessionFamily :execrows
UPDATE auth_sessions
SET
  revoked_at = NOW(),
  revoke_reason = sqlc.arg(revoke_reason),
  updated_at = NOW()
WHERE refresh_token_family_id = sqlc.arg(refresh_token_family_id)
  AND revoked_at IS NULL;

-- name: TouchAuthSession :one
UPDATE auth_sessions
SET
  last_used_at = sqlc.arg(last_used_at),
  idle_expires_at = sqlc.arg(idle_expires_at),
  updated_at = NOW()
WHERE id = sqlc.arg(id)
RETURNING *;
