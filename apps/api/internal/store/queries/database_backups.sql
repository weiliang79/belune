-- name: InsertDatabaseBackup :one
INSERT INTO database_backups (database_id, backup_config_id, target_database)
VALUES ($1, $2, $3)
RETURNING *;

-- name: UpdateDatabaseBackup :exec
UPDATE database_backups
SET finished_at = $2,
    status      = $3,
    local_path  = $4,
    remote_key  = $5,
    size_bytes  = $6,
    error       = $7,
    log         = $8
WHERE id = $1;

-- name: GetDatabaseBackup :one
SELECT * FROM database_backups
WHERE id = $1;

-- name: ListDatabaseBackups :many
SELECT * FROM database_backups
WHERE database_id = $1
ORDER BY started_at DESC
LIMIT $2;

-- name: ListDatabaseBackupsByConfig :many
SELECT * FROM database_backups
WHERE backup_config_id = $1
ORDER BY started_at DESC
LIMIT $2;

-- name: ListProjectDatabaseBackups :many
SELECT b.*, d.name AS database_name, d.slug AS database_slug
FROM database_backups b
JOIN databases d ON d.id = b.database_id
WHERE d.project_id = $1
ORDER BY b.started_at DESC
LIMIT $2;

-- name: DeleteDatabaseBackup :exec
DELETE FROM database_backups WHERE id = $1;

-- name: GetLastSucceededDatabaseBackup :one
SELECT * FROM database_backups
WHERE database_id = $1 AND status = 'succeeded'
ORDER BY finished_at DESC
LIMIT 1;
