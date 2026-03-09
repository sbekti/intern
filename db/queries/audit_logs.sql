-- name: CreateAuditLog :one
INSERT INTO audit_logs (
  actor_user_id,
  actor_username,
  action,
  resource_type,
  resource_id,
  metadata
) VALUES (
  sqlc.narg(actor_user_id),
  sqlc.arg(actor_username),
  sqlc.arg(action),
  sqlc.arg(resource_type),
  sqlc.arg(resource_id),
  sqlc.arg(metadata)
)
RETURNING *;
