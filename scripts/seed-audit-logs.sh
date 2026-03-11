#!/usr/bin/env bash
set -euo pipefail

count="${1:-400}"

if ! [[ "${count}" =~ ^[0-9]+$ ]] || [[ "${count}" -lt 1 ]]; then
  echo "usage: $0 [positive-count]" >&2
  exit 1
fi

container="intern-api-dev-postgres-1"

if ! docker ps --format '{{.Names}}' | grep -qx "${container}"; then
  echo "postgres container ${container} is not running" >&2
  exit 1
fi

docker exec -i "${container}" psql -U postgres -d intern_dev -v seed_count="${count}" <<'SQL'
WITH seed AS (
  SELECT generate_series(1, :seed_count::int) AS n
)
INSERT INTO audit_logs (
  actor_username,
  action,
  resource_type,
  resource_id,
  metadata,
  created_at
)
SELECT
  (ARRAY['alice', 'bob', 'charlie', 'dana'])[((n - 1) % 4) + 1],
  (ARRAY[
    'device.create',
    'device.update',
    'device.delete',
    'vlan.create',
    'vlan.update',
    'auth.session.revoke'
  ])[((n - 1) % 6) + 1],
  (ARRAY[
    'network_device',
    'network_device',
    'network_device',
    'vlan',
    'vlan',
    'auth_session'
  ])[((n - 1) % 6) + 1],
  format('seed-resource-%s', n),
  jsonb_build_object(
    'seed', true,
    'sequence', n,
    'note', 'synthetic audit row',
    'site', 'home-lab'
  ),
  NOW() - make_interval(mins => n)
FROM seed;

SELECT COUNT(*) AS total_audit_logs FROM audit_logs;
SQL
