-- name: CreatePetPhoto :one
INSERT INTO pet_photos (
    pet_id, data, mime_type, size_bytes
) VALUES (
    $1, $2, $3, $4
)
RETURNING id, pet_id, mime_type, size_bytes, created_at;

-- name: GetPetPhoto :one
SELECT id, pet_id, data, mime_type, size_bytes, created_at
FROM pet_photos
WHERE id = $1;

-- name: ListPetPhotos :many
SELECT id, pet_id, mime_type, size_bytes, created_at
FROM pet_photos
WHERE pet_id = $1
ORDER BY created_at DESC;

-- name: DeletePetPhoto :one
DELETE FROM pet_photos
WHERE id = $1
RETURNING pet_id;

-- name: AddPetPhotoURL :one
UPDATE pets
SET photo_urls = array_append(photo_urls, sqlc.arg('photo_url')::text),
    modified_at = NOW(),
    modified_by = $2
WHERE id = $1
RETURNING *;
