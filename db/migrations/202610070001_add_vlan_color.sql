-- +goose Up
ALTER TABLE vlans
ADD COLUMN color text NOT NULL DEFAULT 'default'
CHECK (color IN ('default', 'blue', 'amber', 'violet', 'rose', 'orange', 'cyan', 'slate', 'green'));

-- +goose Down
ALTER TABLE vlans DROP COLUMN color;
