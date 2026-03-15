-- +goose Up
-- +goose StatementBegin
CREATE TABLE presence_observation_points (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  source_key text NOT NULL,
  source_type text NOT NULL CHECK (source_type IN ('radius', 'unifi', 'juniper-snmp', 'juniper-ssh')),
  medium text NOT NULL CHECK (medium IN ('wireless', 'wired')),
  external_id text NOT NULL,
  parent_external_id text NOT NULL DEFAULT '',
  display_name text NOT NULL DEFAULT '',
  location_label text NOT NULL DEFAULT '',
  notes text NOT NULL DEFAULT '',
  ssid text,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  last_seen_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT NOW(),
  updated_at timestamptz NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX presence_observation_points_source_key_external_id_lower_idx
  ON presence_observation_points (source_key, LOWER(external_id));
CREATE INDEX presence_observation_points_source_type_medium_idx
  ON presence_observation_points (source_type, medium);
CREATE INDEX presence_observation_points_location_label_lower_idx
  ON presence_observation_points (LOWER(location_label));

CREATE TABLE presence_clients (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  mac_address text NOT NULL,
  network_device_id uuid REFERENCES network_devices(id) ON UPDATE CASCADE ON DELETE SET NULL,
  status text NOT NULL CHECK (status IN ('online', 'offline')),
  first_seen_at timestamptz NOT NULL,
  last_seen_at timestamptz NOT NULL,
  last_source_key text NOT NULL,
  last_source_type text NOT NULL CHECK (last_source_type IN ('radius', 'unifi', 'juniper-snmp', 'juniper-ssh')),
  last_medium text NOT NULL CHECK (last_medium IN ('wireless', 'wired')),
  last_observation_point_id uuid REFERENCES presence_observation_points(id) ON UPDATE CASCADE ON DELETE SET NULL,
  last_ssid text,
  last_metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT NOW(),
  updated_at timestamptz NOT NULL DEFAULT NOW(),
  CHECK (last_seen_at >= first_seen_at)
);

CREATE UNIQUE INDEX presence_clients_mac_address_lower_idx
  ON presence_clients (LOWER(mac_address));
CREATE INDEX presence_clients_network_device_id_idx
  ON presence_clients (network_device_id);
CREATE INDEX presence_clients_status_last_seen_at_idx
  ON presence_clients (status, last_seen_at DESC);
CREATE INDEX presence_clients_last_source_type_idx
  ON presence_clients (last_source_type, last_medium);
CREATE INDEX presence_clients_last_observation_point_id_idx
  ON presence_clients (last_observation_point_id);

CREATE TABLE presence_sessions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  source_key text NOT NULL,
  source_type text NOT NULL CHECK (source_type IN ('radius', 'unifi', 'juniper-snmp', 'juniper-ssh')),
  medium text NOT NULL CHECK (medium IN ('wireless', 'wired')),
  source_session_key text NOT NULL,
  client_mac_address text NOT NULL,
  network_device_id uuid REFERENCES network_devices(id) ON UPDATE CASCADE ON DELETE SET NULL,
  observation_point_id uuid REFERENCES presence_observation_points(id) ON UPDATE CASCADE ON DELETE SET NULL,
  started_at timestamptz NOT NULL,
  source_updated_at timestamptz NOT NULL,
  last_seen_at timestamptz NOT NULL,
  ended_at timestamptz,
  ssid text,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT NOW(),
  updated_at timestamptz NOT NULL DEFAULT NOW(),
  CHECK (source_updated_at >= started_at),
  CHECK (last_seen_at >= started_at),
  CHECK (ended_at IS NULL OR ended_at >= last_seen_at)
);

CREATE UNIQUE INDEX presence_sessions_source_key_session_key_idx
  ON presence_sessions (source_key, source_session_key);
CREATE INDEX presence_sessions_client_mac_address_lower_idx
  ON presence_sessions (LOWER(client_mac_address));
CREATE INDEX presence_sessions_network_device_id_idx
  ON presence_sessions (network_device_id);
CREATE INDEX presence_sessions_observation_point_id_idx
  ON presence_sessions (observation_point_id);
CREATE INDEX presence_sessions_source_type_medium_idx
  ON presence_sessions (source_type, medium);
CREATE INDEX presence_sessions_started_at_idx
  ON presence_sessions (started_at DESC);
CREATE INDEX presence_sessions_last_seen_at_idx
  ON presence_sessions (last_seen_at DESC);
CREATE INDEX presence_sessions_active_idx
  ON presence_sessions (ended_at NULLS FIRST, last_seen_at DESC);

CREATE TABLE presence_worker_state (
  worker_name text NOT NULL,
  source_key text NOT NULL DEFAULT '',
  state jsonb NOT NULL DEFAULT '{}'::jsonb,
  last_polled_at timestamptz,
  last_succeeded_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT NOW(),
  updated_at timestamptz NOT NULL DEFAULT NOW(),
  PRIMARY KEY (worker_name, source_key)
);

CREATE TRIGGER presence_observation_points_set_updated_at
BEFORE UPDATE ON presence_observation_points
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER presence_clients_set_updated_at
BEFORE UPDATE ON presence_clients
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER presence_sessions_set_updated_at
BEFORE UPDATE ON presence_sessions
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER presence_worker_state_set_updated_at
BEFORE UPDATE ON presence_worker_state
FOR EACH ROW EXECUTE FUNCTION set_updated_at();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS presence_worker_state_set_updated_at ON presence_worker_state;
DROP TRIGGER IF EXISTS presence_sessions_set_updated_at ON presence_sessions;
DROP TRIGGER IF EXISTS presence_clients_set_updated_at ON presence_clients;
DROP TRIGGER IF EXISTS presence_observation_points_set_updated_at ON presence_observation_points;

DROP TABLE IF EXISTS presence_worker_state;
DROP TABLE IF EXISTS presence_sessions;
DROP TABLE IF EXISTS presence_clients;
DROP TABLE IF EXISTS presence_observation_points;
-- +goose StatementEnd
