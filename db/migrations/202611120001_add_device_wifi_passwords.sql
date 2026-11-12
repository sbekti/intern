-- +goose Up
ALTER TABLE network_devices
  ADD COLUMN private_wifi_password_enabled boolean NOT NULL DEFAULT false,
  ADD COLUMN wifi_password_ciphertext bytea,
  ADD CONSTRAINT private_wifi_password_requires_key
    CHECK (NOT private_wifi_password_enabled OR wifi_password_ciphertext IS NOT NULL);

-- +goose Down
ALTER TABLE network_devices
  DROP CONSTRAINT private_wifi_password_requires_key,
  DROP COLUMN wifi_password_ciphertext,
  DROP COLUMN private_wifi_password_enabled;
