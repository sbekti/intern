-- name: ListManagedPresenceClients :many
SELECT
  pc.*,
  COALESCE(pop.external_id, '') AS observation_external_id,
  COALESCE(pop.display_name, '') AS observation_display_name,
  COALESCE(pop.location_label, '') AS observation_location_label
FROM presence_clients pc
LEFT JOIN presence_observation_points pop ON pop.id = pc.last_observation_point_id
WHERE pc.network_device_id IS NOT NULL
ORDER BY pc.last_seen_at DESC;

-- name: GetManagedPresenceClientByDeviceID :one
SELECT
  pc.*,
  COALESCE(pop.external_id, '') AS observation_external_id,
  COALESCE(pop.display_name, '') AS observation_display_name,
  COALESCE(pop.location_label, '') AS observation_location_label
FROM presence_clients pc
LEFT JOIN presence_observation_points pop ON pop.id = pc.last_observation_point_id
WHERE pc.network_device_id = sqlc.arg(network_device_id)
ORDER BY pc.last_seen_at DESC
LIMIT 1;

-- name: CountObservedPresenceClients :one
SELECT COUNT(*)
FROM presence_clients pc
LEFT JOIN network_devices nd ON nd.id = pc.network_device_id
LEFT JOIN presence_observation_points pop ON pop.id = pc.last_observation_point_id
WHERE (sqlc.arg(status) = '' OR pc.status = sqlc.arg(status))
  AND (sqlc.arg(source_type) = '' OR pc.last_source_type = sqlc.arg(source_type))
  AND (sqlc.arg(source_key) = '' OR pc.last_source_key = sqlc.arg(source_key))
  AND (sqlc.arg(medium) = '' OR pc.last_medium = sqlc.arg(medium))
  AND (
    sqlc.arg(location_query) = ''
    OR LOWER(COALESCE(pop.location_label, '')) LIKE '%' || LOWER(sqlc.arg(location_query)) || '%'
  )
  AND (
    sqlc.arg(query) = ''
    OR LOWER(pc.mac_address) LIKE '%' || LOWER(sqlc.arg(query)) || '%'
    OR LOWER(COALESCE(nd.display_name, '')) LIKE '%' || LOWER(sqlc.arg(query)) || '%'
    OR LOWER(COALESCE(pop.location_label, '')) LIKE '%' || LOWER(sqlc.arg(query)) || '%'
    OR LOWER(COALESCE(pop.display_name, '')) LIKE '%' || LOWER(sqlc.arg(query)) || '%'
    OR LOWER(COALESCE(pop.external_id, '')) LIKE '%' || LOWER(sqlc.arg(query)) || '%'
    OR LOWER(COALESCE(pc.last_ssid, '')) LIKE '%' || LOWER(sqlc.arg(query)) || '%'
  );

-- name: ListObservedPresenceClients :many
SELECT
  pc.*,
  COALESCE(nd.display_name, '') AS managed_device_display_name,
  COALESCE(pop.external_id, '') AS observation_external_id,
  COALESCE(pop.display_name, '') AS observation_display_name,
  COALESCE(pop.location_label, '') AS observation_location_label
FROM presence_clients pc
LEFT JOIN network_devices nd ON nd.id = pc.network_device_id
LEFT JOIN presence_observation_points pop ON pop.id = pc.last_observation_point_id
WHERE (sqlc.arg(status) = '' OR pc.status = sqlc.arg(status))
  AND (sqlc.arg(source_type) = '' OR pc.last_source_type = sqlc.arg(source_type))
  AND (sqlc.arg(source_key) = '' OR pc.last_source_key = sqlc.arg(source_key))
  AND (sqlc.arg(medium) = '' OR pc.last_medium = sqlc.arg(medium))
  AND (
    sqlc.arg(location_query) = ''
    OR LOWER(COALESCE(pop.location_label, '')) LIKE '%' || LOWER(sqlc.arg(location_query)) || '%'
  )
  AND (
    sqlc.arg(query) = ''
    OR LOWER(pc.mac_address) LIKE '%' || LOWER(sqlc.arg(query)) || '%'
    OR LOWER(COALESCE(nd.display_name, '')) LIKE '%' || LOWER(sqlc.arg(query)) || '%'
    OR LOWER(COALESCE(pop.location_label, '')) LIKE '%' || LOWER(sqlc.arg(query)) || '%'
    OR LOWER(COALESCE(pop.display_name, '')) LIKE '%' || LOWER(sqlc.arg(query)) || '%'
    OR LOWER(COALESCE(pop.external_id, '')) LIKE '%' || LOWER(sqlc.arg(query)) || '%'
    OR LOWER(COALESCE(pc.last_ssid, '')) LIKE '%' || LOWER(sqlc.arg(query)) || '%'
  )
ORDER BY
  CASE WHEN pc.status = 'online' THEN 0 ELSE 1 END,
  pc.last_seen_at DESC,
  pc.mac_address
LIMIT sqlc.arg(limit_count)
OFFSET sqlc.arg(offset_count);

-- name: CountPresenceObservationPoints :one
SELECT COUNT(*)
FROM presence_observation_points pop
WHERE (sqlc.arg(source_type) = '' OR pop.source_type = sqlc.arg(source_type))
  AND (sqlc.arg(source_key) = '' OR pop.source_key = sqlc.arg(source_key))
  AND (sqlc.arg(medium) = '' OR pop.medium = sqlc.arg(medium))
  AND (
    sqlc.arg(query) = ''
    OR LOWER(pop.external_id) LIKE '%' || LOWER(sqlc.arg(query)) || '%'
    OR LOWER(pop.parent_external_id) LIKE '%' || LOWER(sqlc.arg(query)) || '%'
    OR LOWER(pop.display_name) LIKE '%' || LOWER(sqlc.arg(query)) || '%'
    OR LOWER(pop.location_label) LIKE '%' || LOWER(sqlc.arg(query)) || '%'
    OR LOWER(pop.notes) LIKE '%' || LOWER(sqlc.arg(query)) || '%'
    OR LOWER(COALESCE(pop.ssid, '')) LIKE '%' || LOWER(sqlc.arg(query)) || '%'
  );

-- name: ListPresenceObservationPoints :many
SELECT *
FROM presence_observation_points pop
WHERE (sqlc.arg(source_type) = '' OR pop.source_type = sqlc.arg(source_type))
  AND (sqlc.arg(source_key) = '' OR pop.source_key = sqlc.arg(source_key))
  AND (sqlc.arg(medium) = '' OR pop.medium = sqlc.arg(medium))
  AND (
    sqlc.arg(query) = ''
    OR LOWER(pop.external_id) LIKE '%' || LOWER(sqlc.arg(query)) || '%'
    OR LOWER(pop.parent_external_id) LIKE '%' || LOWER(sqlc.arg(query)) || '%'
    OR LOWER(pop.display_name) LIKE '%' || LOWER(sqlc.arg(query)) || '%'
    OR LOWER(pop.location_label) LIKE '%' || LOWER(sqlc.arg(query)) || '%'
    OR LOWER(pop.notes) LIKE '%' || LOWER(sqlc.arg(query)) || '%'
    OR LOWER(COALESCE(pop.ssid, '')) LIKE '%' || LOWER(sqlc.arg(query)) || '%'
  )
ORDER BY pop.medium, pop.source_key, pop.external_id
LIMIT sqlc.arg(limit_count)
OFFSET sqlc.arg(offset_count);

-- name: GetPresenceObservationPointByID :one
SELECT *
FROM presence_observation_points
WHERE id = sqlc.arg(id)
LIMIT 1;

-- name: UpdatePresenceObservationPointAdmin :one
UPDATE presence_observation_points
SET
  location_label = sqlc.arg(location_label),
  notes = sqlc.arg(notes)
WHERE id = sqlc.arg(id)
RETURNING *;
