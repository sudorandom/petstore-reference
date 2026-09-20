-- +goose Up
CREATE TABLE IF NOT EXISTS pets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    species VARCHAR(50) NOT NULL,
    birth_date DATE NOT NULL,
    birth_date_estimated BOOLEAN NOT NULL DEFAULT FALSE,
    status VARCHAR(30) NOT NULL DEFAULT 'PET_STATUS_AVAILABLE' CHECK (status IN ('PET_STATUS_AVAILABLE', 'PET_STATUS_PENDING', 'PET_STATUS_ADOPTED', 'AVAILABLE', 'PENDING', 'ADOPTED')),
    photo_urls TEXT[] NOT NULL DEFAULT '{}',
    tags TEXT[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    modified_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(255) NOT NULL,
    modified_by VARCHAR(255) NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_pets_status ON pets(status);
CREATE INDEX IF NOT EXISTS idx_pets_species ON pets(species);

CREATE TABLE IF NOT EXISTS pet_photos (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pet_id UUID NOT NULL REFERENCES pets(id) ON DELETE CASCADE,
    data BYTEA NOT NULL,
    mime_type VARCHAR(50) NOT NULL,
    size_bytes INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_pet_photos_pet_id ON pet_photos(pet_id);

-- +goose Down
DROP TABLE IF EXISTS pet_photos;
DROP TABLE IF EXISTS pets;
