-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
  NEW.updated_at = NOW();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TABLE users (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  username text NOT NULL UNIQUE,
  name text NOT NULL,
  email text NOT NULL,
  groups text[] NOT NULL DEFAULT ARRAY[]::text[],
  created_at timestamptz NOT NULL DEFAULT NOW(),
  updated_at timestamptz NOT NULL DEFAULT NOW()
);

CREATE TABLE user_preferences (
  user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  timezone text,
  locale text,
  created_at timestamptz NOT NULL DEFAULT NOW(),
  updated_at timestamptz NOT NULL DEFAULT NOW()
);

CREATE TABLE vlans (
  vlan_id integer PRIMARY KEY CHECK (vlan_id BETWEEN 1 AND 4094),
  name text NOT NULL,
  description text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT NOW(),
  updated_at timestamptz NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX vlans_name_lower_idx ON vlans (LOWER(name));

CREATE TABLE network_devices (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  mac_address text NOT NULL,
  display_name text NOT NULL,
  vlan_id integer NOT NULL REFERENCES vlans(vlan_id) ON UPDATE CASCADE ON DELETE RESTRICT,
  created_by_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
  updated_by_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT NOW(),
  updated_at timestamptz NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX network_devices_mac_address_lower_idx ON network_devices (LOWER(mac_address));
CREATE INDEX network_devices_vlan_id_idx ON network_devices (vlan_id);

CREATE TABLE auth_device_authorizations (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  device_code text NOT NULL UNIQUE,
  user_code text NOT NULL UNIQUE,
  client_name text NOT NULL DEFAULT 'internctl',
  requested_scopes text[] NOT NULL DEFAULT ARRAY[]::text[],
  approved_by_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
  approved_at timestamptz,
  last_polled_at timestamptz,
  expires_at timestamptz NOT NULL,
  status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'exchanged', 'expired', 'denied')),
  created_at timestamptz NOT NULL DEFAULT NOW(),
  updated_at timestamptz NOT NULL DEFAULT NOW()
);

CREATE INDEX auth_device_authorizations_status_idx ON auth_device_authorizations (status);
CREATE INDEX auth_device_authorizations_expires_at_idx ON auth_device_authorizations (expires_at);

CREATE TABLE auth_sessions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  client_name text NOT NULL DEFAULT 'internctl',
  user_agent text NOT NULL DEFAULT '',
  refresh_token_hash text NOT NULL,
  refresh_token_family_id uuid NOT NULL DEFAULT gen_random_uuid(),
  last_used_at timestamptz,
  expires_at timestamptz NOT NULL,
  idle_expires_at timestamptz NOT NULL,
  revoked_at timestamptz,
  revoke_reason text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT NOW(),
  updated_at timestamptz NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX auth_sessions_refresh_token_hash_idx ON auth_sessions (refresh_token_hash);
CREATE INDEX auth_sessions_user_id_idx ON auth_sessions (user_id);
CREATE INDEX auth_sessions_refresh_token_family_id_idx ON auth_sessions (refresh_token_family_id);
CREATE INDEX auth_sessions_expires_at_idx ON auth_sessions (expires_at);
CREATE INDEX auth_sessions_idle_expires_at_idx ON auth_sessions (idle_expires_at);

CREATE TABLE audit_logs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  actor_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
  actor_username text NOT NULL,
  action text NOT NULL,
  resource_type text NOT NULL,
  resource_id text NOT NULL,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT NOW()
);

CREATE INDEX audit_logs_actor_user_id_idx ON audit_logs (actor_user_id);
CREATE INDEX audit_logs_resource_idx ON audit_logs (resource_type, resource_id);
CREATE INDEX audit_logs_created_at_idx ON audit_logs (created_at DESC);

INSERT INTO vlans (vlan_id, name, description)
VALUES
  (1, 'trusted', 'Trusted devices'),
  (10, 'guest', 'Guest devices'),
  (20, 'iot', 'IoT devices');

CREATE TRIGGER users_set_updated_at
BEFORE UPDATE ON users
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER user_preferences_set_updated_at
BEFORE UPDATE ON user_preferences
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER vlans_set_updated_at
BEFORE UPDATE ON vlans
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER network_devices_set_updated_at
BEFORE UPDATE ON network_devices
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER auth_device_authorizations_set_updated_at
BEFORE UPDATE ON auth_device_authorizations
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER auth_sessions_set_updated_at
BEFORE UPDATE ON auth_sessions
FOR EACH ROW EXECUTE FUNCTION set_updated_at();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS auth_sessions_set_updated_at ON auth_sessions;
DROP TRIGGER IF EXISTS auth_device_authorizations_set_updated_at ON auth_device_authorizations;
DROP TRIGGER IF EXISTS network_devices_set_updated_at ON network_devices;
DROP TRIGGER IF EXISTS vlans_set_updated_at ON vlans;
DROP TRIGGER IF EXISTS user_preferences_set_updated_at ON user_preferences;
DROP TRIGGER IF EXISTS users_set_updated_at ON users;

DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS auth_sessions;
DROP TABLE IF EXISTS auth_device_authorizations;
DROP TABLE IF EXISTS network_devices;
DROP TABLE IF EXISTS vlans;
DROP TABLE IF EXISTS user_preferences;
DROP TABLE IF EXISTS users;

DROP FUNCTION IF EXISTS set_updated_at();
DROP EXTENSION IF EXISTS pgcrypto;
-- +goose StatementEnd
