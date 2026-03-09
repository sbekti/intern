-- name: UpsertUserByUsername :one
INSERT INTO users (
  username,
  name,
  email,
  groups
) VALUES (
  sqlc.arg(username),
  sqlc.arg(name),
  sqlc.arg(email),
  sqlc.arg(groups)
)
ON CONFLICT (username) DO UPDATE SET
  name = EXCLUDED.name,
  email = EXCLUDED.email,
  groups = EXCLUDED.groups,
  updated_at = NOW()
RETURNING *;

-- name: GetUserByID :one
SELECT *
FROM users
WHERE id = sqlc.arg(id)
LIMIT 1;

-- name: GetUserByUsername :one
SELECT *
FROM users
WHERE username = sqlc.arg(username)
LIMIT 1;
