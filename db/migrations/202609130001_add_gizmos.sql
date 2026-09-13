-- +goose Up
CREATE TABLE gizmos (
  network_device_id uuid PRIMARY KEY REFERENCES network_devices(id) ON DELETE CASCADE,
  kiosk_url varchar(2048),
  created_at timestamptz NOT NULL DEFAULT NOW(),
  updated_at timestamptz NOT NULL DEFAULT NOW()
);

CREATE TRIGGER gizmos_set_updated_at
BEFORE UPDATE ON gizmos
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS gizmos_set_updated_at ON gizmos;
DROP TABLE IF EXISTS gizmos;
