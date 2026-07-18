-- name: ListDatabasesByProject :many
SELECT * FROM databases WHERE project_id = $1 ORDER BY created_at DESC;

-- name: GetDatabase :one
SELECT * FROM databases WHERE id = $1;

-- name: CreateDatabase :one
INSERT INTO databases (
    project_id, type, name, slug, version, status, internal_host, internal_port,
    credentials_encrypted, image, container_port, data_dir, backup_mode,
    backup_command, restore_command
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
RETURNING *;

-- name: UpdateDatabaseStatus :one
UPDATE databases SET status = $2 WHERE id = $1 RETURNING *;

-- name: UpdateDatabaseAfterProvision :one
UPDATE databases SET status = $2, internal_host = $3, internal_port = $4, host_port = $5 WHERE id = $1 RETURNING *;

-- name: UpdateDatabaseResources :one
UPDATE databases SET cpu_limit = $2, memory_limit = $3 WHERE id = $1 RETURNING *;

-- name: UpdateDatabaseVersion :one
UPDATE databases SET version = $2 WHERE id = $1 RETURNING *;

-- name: UpdateDatabaseImageDigest :exec
-- Pins (or clears, with NULL) the resolved @sha256 image digest so recreates
-- reuse the exact image. Upgrade clears it so the target tag is re-pinned.
UPDATE databases SET image_digest = $2 WHERE id = $1;

-- name: ListDatabasesByStatus :many
SELECT * FROM databases WHERE status = $1;

-- name: ListAllDatabases :many
SELECT * FROM databases;

-- name: DeleteDatabase :exec
DELETE FROM databases WHERE id = $1;

-- name: UpdateDatabaseSlug :exec
UPDATE databases SET slug = $2 WHERE id = $1;

-- name: CountDatabases :one
SELECT count(*) FROM databases;

-- name: GetDatabaseOwnerUserID :one
SELECT p.user_id FROM databases d
JOIN projects p ON p.id = d.project_id
WHERE d.id = $1;

-- name: UpdateDatabaseSource :exec
UPDATE databases SET source_kind = $2, source_ref = $3, updated_at = NOW()
WHERE id = $1;
