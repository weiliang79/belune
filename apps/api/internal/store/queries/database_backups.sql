-- name: InsertDatabaseBackup :one
INSERT INTO database_backups (database_id)
VALUES ($1)
RETURNING *;

-- name: UpdateDatabaseBackup :exec
UPDATE database_backups
SET finished_at = $2,
    status      = $3,
    local_path  = $4,
    remote_key  = $5,
    size_bytes  = $6,
    error       = $7
WHERE id = $1;

-- name: GetDatabaseBackup :one
SELECT * FROM database_backups
WHERE id = $1;

-- name: ListDatabaseBackups :many
SELECT * FROM database_backups
WHERE database_id = $1
ORDER BY started_at DESC
LIMIT $2;

-- name: GetLastSucceededDatabaseBackup :one
SELECT * FROM database_backups
WHERE database_id = $1 AND status = 'succeeded'
ORDER BY finished_at DESC
LIMIT 1;
