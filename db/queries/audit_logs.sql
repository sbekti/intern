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

-- name: ListAuditLogs :many
SELECT *
FROM audit_logs
WHERE (sqlc.arg(action) = '' OR action = sqlc.arg(action))
  AND (sqlc.arg(resource_type) = '' OR resource_type = sqlc.arg(resource_type))
  AND (sqlc.arg(resource_id) = '' OR resource_id = sqlc.arg(resource_id))
  AND (sqlc.arg(actor_username) = '' OR actor_username = sqlc.arg(actor_username))
ORDER BY
  CASE WHEN sqlc.arg(sort_by)::text = 'actor_username' AND sqlc.arg(sort_dir)::text = 'asc' THEN actor_username END ASC,
  CASE WHEN sqlc.arg(sort_by)::text = 'actor_username' AND sqlc.arg(sort_dir)::text = 'desc' THEN actor_username END DESC,
  CASE WHEN sqlc.arg(sort_by)::text = 'action' AND sqlc.arg(sort_dir)::text = 'asc' THEN action END ASC,
  CASE WHEN sqlc.arg(sort_by)::text = 'action' AND sqlc.arg(sort_dir)::text = 'desc' THEN action END DESC,
  CASE WHEN sqlc.arg(sort_by)::text = 'resource' AND sqlc.arg(sort_dir)::text = 'asc' THEN resource_type END ASC,
  CASE WHEN sqlc.arg(sort_by)::text = 'resource' AND sqlc.arg(sort_dir)::text = 'desc' THEN resource_type END DESC,
  CASE WHEN sqlc.arg(sort_by)::text = 'resource' AND sqlc.arg(sort_dir)::text = 'asc' THEN resource_id END ASC,
  CASE WHEN sqlc.arg(sort_by)::text = 'resource' AND sqlc.arg(sort_dir)::text = 'desc' THEN resource_id END DESC,
  CASE WHEN sqlc.arg(sort_by)::text = 'created_at' AND sqlc.arg(sort_dir)::text = 'asc' THEN created_at END ASC,
  CASE WHEN sqlc.arg(sort_by)::text = 'created_at' AND sqlc.arg(sort_dir)::text = 'asc' THEN id END ASC,
  created_at DESC, id DESC
LIMIT sqlc.arg(limit_count)
OFFSET sqlc.arg(offset_count);

-- name: CountAuditLogs :one
SELECT COUNT(*)
FROM audit_logs
WHERE (sqlc.arg(action) = '' OR action = sqlc.arg(action))
  AND (sqlc.arg(resource_type) = '' OR resource_type = sqlc.arg(resource_type))
  AND (sqlc.arg(resource_id) = '' OR resource_id = sqlc.arg(resource_id))
  AND (sqlc.arg(actor_username) = '' OR actor_username = sqlc.arg(actor_username));
