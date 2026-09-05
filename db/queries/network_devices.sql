-- name: ListNetworkDevices :many
SELECT sqlc.embed(network_devices), sqlc.embed(vlans)
FROM network_devices
JOIN vlans ON vlans.vlan_id = network_devices.vlan_id
ORDER BY network_devices.display_name, network_devices.id;

-- name: GetNetworkDeviceByID :one
SELECT *
FROM network_devices
WHERE id = sqlc.arg(id)
LIMIT 1;

-- name: CreateNetworkDevice :one
INSERT INTO network_devices (
  mac_address,
  display_name,
  disabled,
  vlan_id,
  created_by_user_id,
  updated_by_user_id
) VALUES (
  sqlc.arg(mac_address),
  sqlc.arg(display_name),
  sqlc.arg(disabled),
  sqlc.arg(vlan_id),
  sqlc.narg(created_by_user_id),
  sqlc.narg(updated_by_user_id)
)
RETURNING *;

-- name: UpdateNetworkDevice :one
UPDATE network_devices
SET
  mac_address = sqlc.arg(mac_address),
  display_name = sqlc.arg(display_name),
  disabled = sqlc.arg(disabled),
  vlan_id = sqlc.arg(vlan_id),
  updated_by_user_id = sqlc.narg(updated_by_user_id),
  updated_at = NOW()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: DeleteNetworkDevice :exec
DELETE FROM network_devices
WHERE id = sqlc.arg(id);
