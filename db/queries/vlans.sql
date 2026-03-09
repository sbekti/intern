-- name: ListVlans :many
SELECT *
FROM vlans
ORDER BY vlan_id, id;

-- name: CountVlans :one
SELECT COUNT(*)
FROM vlans;

-- name: GetVlanByID :one
SELECT *
FROM vlans
WHERE id = sqlc.arg(id)
LIMIT 1;

-- name: GetVlanByName :one
SELECT *
FROM vlans
WHERE LOWER(name) = LOWER(sqlc.arg(name))
LIMIT 1;

-- name: CreateVlan :one
INSERT INTO vlans (
  name,
  vlan_id,
  description,
  is_active
) VALUES (
  sqlc.arg(name),
  sqlc.arg(vlan_id),
  sqlc.arg(description),
  sqlc.arg(is_active)
)
RETURNING *;

-- name: UpdateVlan :one
UPDATE vlans
SET
  name = sqlc.arg(name),
  vlan_id = sqlc.arg(vlan_id),
  description = sqlc.arg(description),
  is_active = sqlc.arg(is_active),
  updated_at = NOW()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: DeleteVlan :exec
DELETE FROM vlans
WHERE id = sqlc.arg(id);
