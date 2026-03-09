-- name: UpsertUserPreferences :one
INSERT INTO user_preferences (
  user_id,
  timezone,
  locale
) VALUES (
  sqlc.arg(user_id),
  sqlc.narg(timezone),
  sqlc.narg(locale)
)
ON CONFLICT (user_id) DO UPDATE SET
  timezone = EXCLUDED.timezone,
  locale = EXCLUDED.locale,
  updated_at = NOW()
RETURNING *;
