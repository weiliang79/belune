-- name: CreateApplicationFileMount :one
INSERT INTO application_file_mounts (application_id, mount_path, content_encrypted, is_secret, file_mode)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetApplicationFileMount :one
SELECT * FROM application_file_mounts WHERE id = $1;

-- name: ListApplicationFileMounts :many
SELECT * FROM application_file_mounts
WHERE application_id = $1
ORDER BY mount_path;

-- name: UpdateApplicationFileMount :one
UPDATE application_file_mounts
SET content_encrypted = $2,
    is_secret         = $3,
    file_mode         = $4,
    updated_at        = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteApplicationFileMount :exec
DELETE FROM application_file_mounts WHERE id = $1;
