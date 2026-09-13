-- name: ListGizmos :many
SELECT sqlc.embed(gizmos), sqlc.embed(network_devices), sqlc.embed(vlans)
FROM gizmos
JOIN network_devices ON network_devices.id = gizmos.network_device_id
JOIN vlans ON vlans.vlan_id = network_devices.vlan_id
ORDER BY network_devices.display_name, network_devices.id;

-- name: GetGizmoByNetworkDeviceID :one
SELECT sqlc.embed(gizmos), sqlc.embed(network_devices), sqlc.embed(vlans)
FROM gizmos
JOIN network_devices ON network_devices.id = gizmos.network_device_id
JOIN vlans ON vlans.vlan_id = network_devices.vlan_id
WHERE gizmos.network_device_id = sqlc.arg(network_device_id)
LIMIT 1;

-- name: GetKioskURLByMAC :one
SELECT gizmos.kiosk_url
FROM gizmos
JOIN network_devices ON network_devices.id = gizmos.network_device_id
WHERE network_devices.mac_address = sqlc.arg(mac_address)
  AND network_devices.disabled = false
  AND gizmos.kiosk_url IS NOT NULL
LIMIT 1;

-- name: CreateGizmo :one
INSERT INTO gizmos (
  network_device_id,
  kiosk_url
) VALUES (
  sqlc.arg(network_device_id),
  sqlc.narg(kiosk_url)
)
RETURNING *;

-- name: UpdateGizmo :one
UPDATE gizmos
SET kiosk_url = sqlc.narg(kiosk_url)
WHERE network_device_id = sqlc.arg(network_device_id)
RETURNING *;

-- name: DeleteGizmo :exec
DELETE FROM gizmos
WHERE network_device_id = sqlc.arg(network_device_id);
