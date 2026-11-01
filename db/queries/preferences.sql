-- name: GetApplicationPreferences :one
SELECT preferences FROM application_preferences WHERE id = true;

-- name: MergeApplicationPreferences :one
INSERT INTO application_preferences (id, preferences)
VALUES (true, sqlc.arg(patch)::jsonb)
ON CONFLICT (id) DO UPDATE
SET preferences = application_preferences.preferences || EXCLUDED.preferences
RETURNING preferences;
