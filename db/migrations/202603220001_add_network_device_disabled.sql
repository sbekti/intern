-- +goose Up
ALTER TABLE network_devices
  ADD COLUMN disabled boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE network_devices
  DROP COLUMN disabled;
