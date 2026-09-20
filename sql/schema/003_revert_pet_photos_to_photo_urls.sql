-- +goose Up
ALTER TABLE pets ADD COLUMN IF NOT EXISTS photo_urls TEXT[] NOT NULL DEFAULT '{}';
DROP TABLE IF EXISTS pet_photos;

-- +goose Down
CREATE TABLE IF NOT EXISTS pet_photos (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pet_id UUID NOT NULL REFERENCES pets(id) ON DELETE CASCADE,
    data BYTEA NOT NULL,
    mime_type VARCHAR(50) NOT NULL,
    size_bytes INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_pet_photos_pet_id ON pet_photos(pet_id);
ALTER TABLE pets DROP COLUMN IF EXISTS photo_urls;
