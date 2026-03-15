-- name: GetPresenceWorkerState :one
SELECT *
FROM presence_worker_state
WHERE worker_name = sqlc.arg(worker_name)
  AND source_key = sqlc.arg(source_key)
LIMIT 1;

-- name: UpsertPresenceWorkerState :one
INSERT INTO presence_worker_state (
  worker_name,
  source_key,
  state,
  last_polled_at,
  last_succeeded_at
) VALUES (
  sqlc.arg(worker_name),
  sqlc.arg(source_key),
  sqlc.arg(state),
  sqlc.narg(last_polled_at),
  sqlc.narg(last_succeeded_at)
)
ON CONFLICT (worker_name, source_key) DO UPDATE SET
  state = EXCLUDED.state,
  last_polled_at = EXCLUDED.last_polled_at,
  last_succeeded_at = EXCLUDED.last_succeeded_at
RETURNING *;

-- name: UpsertPresenceObservationPoint :one
INSERT INTO presence_observation_points (
  source_key,
  source_type,
  medium,
  external_id,
  parent_external_id,
  display_name,
  ssid,
  metadata,
  last_seen_at
) VALUES (
  sqlc.arg(source_key),
  sqlc.arg(source_type),
  sqlc.arg(medium),
  sqlc.arg(external_id),
  sqlc.arg(parent_external_id),
  sqlc.arg(display_name),
  sqlc.narg(ssid),
  sqlc.arg(metadata),
  sqlc.narg(last_seen_at)
)
ON CONFLICT (source_key, LOWER(external_id)) DO UPDATE SET
  source_type = EXCLUDED.source_type,
  medium = EXCLUDED.medium,
  parent_external_id = CASE
    WHEN EXCLUDED.parent_external_id <> '' THEN EXCLUDED.parent_external_id
    ELSE presence_observation_points.parent_external_id
  END,
  display_name = CASE
    WHEN EXCLUDED.display_name <> '' THEN EXCLUDED.display_name
    ELSE presence_observation_points.display_name
  END,
  ssid = COALESCE(EXCLUDED.ssid, presence_observation_points.ssid),
  metadata = presence_observation_points.metadata || EXCLUDED.metadata,
  last_seen_at = CASE
    WHEN presence_observation_points.last_seen_at IS NULL THEN EXCLUDED.last_seen_at
    WHEN EXCLUDED.last_seen_at IS NULL THEN presence_observation_points.last_seen_at
    WHEN EXCLUDED.last_seen_at > presence_observation_points.last_seen_at THEN EXCLUDED.last_seen_at
    ELSE presence_observation_points.last_seen_at
  END
RETURNING *;

-- name: UpsertPresenceClient :one
INSERT INTO presence_clients (
  mac_address,
  network_device_id,
  status,
  first_seen_at,
  last_seen_at,
  last_source_key,
  last_source_type,
  last_medium,
  last_observation_point_id,
  last_ssid,
  last_metadata
) VALUES (
  sqlc.arg(mac_address),
  sqlc.narg(network_device_id),
  sqlc.arg(status),
  sqlc.arg(first_seen_at),
  sqlc.arg(last_seen_at),
  sqlc.arg(last_source_key),
  sqlc.arg(last_source_type),
  sqlc.arg(last_medium),
  sqlc.narg(last_observation_point_id),
  sqlc.narg(last_ssid),
  sqlc.arg(last_metadata)
)
ON CONFLICT (LOWER(mac_address)) DO UPDATE SET
  network_device_id = COALESCE(EXCLUDED.network_device_id, presence_clients.network_device_id),
  status = CASE
    WHEN EXCLUDED.last_seen_at >= presence_clients.last_seen_at THEN EXCLUDED.status
    ELSE presence_clients.status
  END,
  first_seen_at = LEAST(presence_clients.first_seen_at, EXCLUDED.first_seen_at),
  last_seen_at = GREATEST(presence_clients.last_seen_at, EXCLUDED.last_seen_at),
  last_source_key = CASE
    WHEN EXCLUDED.last_seen_at >= presence_clients.last_seen_at THEN EXCLUDED.last_source_key
    ELSE presence_clients.last_source_key
  END,
  last_source_type = CASE
    WHEN EXCLUDED.last_seen_at >= presence_clients.last_seen_at THEN EXCLUDED.last_source_type
    ELSE presence_clients.last_source_type
  END,
  last_medium = CASE
    WHEN EXCLUDED.last_seen_at >= presence_clients.last_seen_at THEN EXCLUDED.last_medium
    ELSE presence_clients.last_medium
  END,
  last_observation_point_id = CASE
    WHEN EXCLUDED.last_seen_at >= presence_clients.last_seen_at AND EXCLUDED.last_observation_point_id IS NOT NULL THEN EXCLUDED.last_observation_point_id
    ELSE presence_clients.last_observation_point_id
  END,
  last_ssid = CASE
    WHEN EXCLUDED.last_seen_at >= presence_clients.last_seen_at AND EXCLUDED.last_ssid IS NOT NULL THEN EXCLUDED.last_ssid
    ELSE presence_clients.last_ssid
  END,
  last_metadata = presence_clients.last_metadata || EXCLUDED.last_metadata
RETURNING *;

-- name: UpsertPresenceSession :one
INSERT INTO presence_sessions (
  source_key,
  source_type,
  medium,
  source_session_key,
  client_mac_address,
  network_device_id,
  observation_point_id,
  started_at,
  source_updated_at,
  last_seen_at,
  ended_at,
  ssid,
  metadata
) VALUES (
  sqlc.arg(source_key),
  sqlc.arg(source_type),
  sqlc.arg(medium),
  sqlc.arg(source_session_key),
  sqlc.arg(client_mac_address),
  sqlc.narg(network_device_id),
  sqlc.narg(observation_point_id),
  sqlc.arg(started_at),
  sqlc.arg(source_updated_at),
  sqlc.arg(last_seen_at),
  sqlc.narg(ended_at),
  sqlc.narg(ssid),
  sqlc.arg(metadata)
)
ON CONFLICT (source_key, source_session_key) DO UPDATE SET
  network_device_id = COALESCE(EXCLUDED.network_device_id, presence_sessions.network_device_id),
  observation_point_id = COALESCE(EXCLUDED.observation_point_id, presence_sessions.observation_point_id),
  started_at = LEAST(presence_sessions.started_at, EXCLUDED.started_at),
  source_updated_at = GREATEST(presence_sessions.source_updated_at, EXCLUDED.source_updated_at),
  last_seen_at = GREATEST(presence_sessions.last_seen_at, EXCLUDED.last_seen_at),
  ended_at = CASE
    WHEN presence_sessions.ended_at IS NULL THEN EXCLUDED.ended_at
    WHEN EXCLUDED.ended_at IS NULL THEN presence_sessions.ended_at
    WHEN EXCLUDED.ended_at > presence_sessions.ended_at THEN EXCLUDED.ended_at
    ELSE presence_sessions.ended_at
  END,
  ssid = COALESCE(EXCLUDED.ssid, presence_sessions.ssid),
  metadata = presence_sessions.metadata || EXCLUDED.metadata
RETURNING *;

-- name: GetLatestOpenPresenceSessionForClientMedium :one
SELECT
  ps.*,
  COALESCE(pop.external_id, '') AS observation_external_id,
  COALESCE(pop.parent_external_id, '') AS observation_parent_external_id
FROM presence_sessions ps
LEFT JOIN presence_observation_points pop ON pop.id = ps.observation_point_id
WHERE LOWER(ps.client_mac_address) = LOWER(sqlc.arg(client_mac_address))
  AND ps.medium = sqlc.arg(medium)
  AND ps.ended_at IS NULL
ORDER BY ps.last_seen_at DESC, ps.started_at DESC
LIMIT 1;

-- name: ListStaleOnlinePresenceClientsBySource :many
SELECT *
FROM presence_clients
WHERE status = 'online'
  AND last_source_key = sqlc.arg(source_key)
  AND last_source_type = sqlc.arg(source_type)
  AND last_medium = sqlc.arg(medium)
  AND last_seen_at <= sqlc.arg(seen_before)
ORDER BY last_seen_at ASC;

-- name: UpdatePresenceSessionActivity :one
UPDATE presence_sessions
SET
  network_device_id = COALESCE(sqlc.narg(network_device_id), network_device_id),
  observation_point_id = COALESCE(sqlc.narg(observation_point_id), observation_point_id),
  source_updated_at = GREATEST(source_updated_at, sqlc.arg(source_updated_at)),
  last_seen_at = GREATEST(last_seen_at, sqlc.arg(last_seen_at)),
  ssid = COALESCE(sqlc.narg(ssid), ssid),
  metadata = metadata || sqlc.arg(metadata)
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: ClosePresenceSession :one
UPDATE presence_sessions
SET
  network_device_id = COALESCE(sqlc.narg(network_device_id), network_device_id),
  observation_point_id = COALESCE(sqlc.narg(observation_point_id), observation_point_id),
  source_updated_at = GREATEST(source_updated_at, sqlc.arg(source_updated_at)),
  last_seen_at = GREATEST(last_seen_at, sqlc.arg(last_seen_at)),
  ended_at = CASE
    WHEN ended_at IS NULL THEN sqlc.arg(ended_at)
    WHEN sqlc.arg(ended_at) > ended_at THEN sqlc.arg(ended_at)
    ELSE ended_at
  END,
  ssid = COALESCE(sqlc.narg(ssid), ssid),
  metadata = metadata || sqlc.arg(metadata)
WHERE id = sqlc.arg(id)
RETURNING *;
