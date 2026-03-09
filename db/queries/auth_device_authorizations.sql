-- name: CreateAuthDeviceAuthorization :one
INSERT INTO auth_device_authorizations (
  device_code,
  user_code,
  client_name,
  requested_scopes,
  expires_at,
  status
) VALUES (
  sqlc.arg(device_code),
  sqlc.arg(user_code),
  sqlc.arg(client_name),
  sqlc.arg(requested_scopes),
  sqlc.arg(expires_at),
  sqlc.arg(status)
)
RETURNING *;

-- name: GetAuthDeviceAuthorizationByDeviceCode :one
SELECT *
FROM auth_device_authorizations
WHERE device_code = sqlc.arg(device_code)
LIMIT 1;

-- name: GetAuthDeviceAuthorizationByUserCode :one
SELECT *
FROM auth_device_authorizations
WHERE user_code = sqlc.arg(user_code)
LIMIT 1;

-- name: UpdateAuthDeviceAuthorizationStatus :one
UPDATE auth_device_authorizations
SET
  status = sqlc.arg(status),
  approved_by_user_id = sqlc.narg(approved_by_user_id),
  approved_at = sqlc.narg(approved_at),
  last_polled_at = sqlc.narg(last_polled_at),
  updated_at = NOW()
WHERE id = sqlc.arg(id)
RETURNING *;
