-- name: ListVlans :many
SELECT *
FROM vlans
ORDER BY vlan_id;

-- name: GetVlanByVlanID :one
SELECT *
FROM vlans
WHERE vlan_id = sqlc.arg(vlan_id)
LIMIT 1;

-- name: CreateVlan :one
INSERT INTO vlans (
  vlan_id,
  name,
  description
) VALUES (
  sqlc.arg(vlan_id),
  sqlc.arg(name),
  sqlc.arg(description)
)
RETURNING *;

-- name: UpdateVlan :one
UPDATE vlans
SET
  vlan_id = sqlc.arg(vlan_id),
  name = sqlc.arg(name),
  description = sqlc.arg(description),
  updated_at = NOW()
WHERE vlan_id = sqlc.arg(current_vlan_id)
RETURNING *;

-- name: DeleteVlan :exec
DELETE FROM vlans
WHERE vlan_id = sqlc.arg(vlan_id);
