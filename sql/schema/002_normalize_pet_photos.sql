-- +goose Up
ALTER TABLE pets DROP COLUMN IF EXISTS photo_urls;

-- +goose Down
ALTER TABLE pets ADD COLUMN IF NOT EXISTS photo_urls TEXT[] NOT NULL DEFAULT '{}';
