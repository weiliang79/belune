-- name: CreateApplicationVolume :one
INSERT INTO application_volumes (application_id, name, mount_path)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetApplicationVolume :one
SELECT * FROM application_volumes WHERE id = $1;

-- name: ListApplicationVolumes :many
SELECT * FROM application_volumes
WHERE application_id = $1
ORDER BY mount_path;

-- name: DeleteApplicationVolume :exec
DELETE FROM application_volumes WHERE id = $1;
