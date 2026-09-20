-- +goose Up
ALTER TABLE pets ALTER COLUMN birth_date DROP NOT NULL;

-- +goose Down
UPDATE pets SET birth_date = CURRENT_DATE WHERE birth_date IS NULL;
ALTER TABLE pets ALTER COLUMN birth_date SET NOT NULL;
