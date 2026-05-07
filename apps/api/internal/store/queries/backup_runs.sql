-- name: InsertBackupRun :one
INSERT INTO backup_runs DEFAULT VALUES
RETURNING id, started_at, finished_at, status, local_path, remote_key, size_bytes, error;

-- name: UpdateBackupRun :exec
UPDATE backup_runs
SET finished_at = $2,
    status      = $3,
    local_path  = $4,
    remote_key  = $5,
    size_bytes  = $6,
    error       = $7
WHERE id = $1;

-- name: ListBackupRuns :many
SELECT id, started_at, finished_at, status, local_path, remote_key, size_bytes, error
FROM backup_runs
ORDER BY started_at DESC
LIMIT $1;

-- name: GetLastSucceededBackupRun :one
SELECT id, started_at, finished_at, status, local_path, remote_key, size_bytes, error
FROM backup_runs
WHERE status = 'succeeded'
ORDER BY finished_at DESC
LIMIT 1;

-- name: GetLastBackupRun :one
SELECT id, started_at, finished_at, status, local_path, remote_key, size_bytes, error
FROM backup_runs
ORDER BY started_at DESC
LIMIT 1;
