-- +goose Up
DROP TABLE IF EXISTS nasreload;
DROP TABLE IF EXISTS nas;
DROP TABLE IF EXISTS radpostauth;
DROP TABLE IF EXISTS radusergroup;
DROP TABLE IF EXISTS radreply;
DROP TABLE IF EXISTS radgroupreply;
DROP TABLE IF EXISTS radgroupcheck;
DROP TABLE IF EXISTS radcheck;
DROP TABLE IF EXISTS radacct;

-- +goose Down
-- The retired RADIUS data is intentionally not recreated.
