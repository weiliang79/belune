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

-- name: ListApplicationVolumeOwners :many
-- Maps each application volume to its owning app so the admin Docker page can
-- link Docker volumes back to the application. Volumes carry no application-id
-- label, but the Docker volume name is reconstructable from (application_id,
-- name) via naming.AppVolumeName, so ownership is resolved from this table.
SELECT av.application_id, av.name, a.name AS application_name, a.project_id
FROM application_volumes av
JOIN applications a ON a.id = av.application_id;
