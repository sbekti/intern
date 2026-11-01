-- +goose Up
CREATE TABLE application_preferences (
  id boolean PRIMARY KEY DEFAULT true CHECK (id),
  preferences jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(preferences) = 'object')
);

-- +goose Down
DROP TABLE application_preferences;
