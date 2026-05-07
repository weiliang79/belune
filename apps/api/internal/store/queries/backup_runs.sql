-- name: InsertBackupRun :one
INSERT INTO backup_runs DEFAULT VALUES
RETURNING id, started_at, finished_at, status, remote_key, size_bytes, error;

-- name: UpdateBackupRun :exec
UPDATE backup_runs
SET finished_at = $2,
    status      = $3,
    remote_key  = $4,
    size_bytes  = $5,
    error       = $6
WHERE id = $1;

-- name: ListBackupRuns :many
SELECT id, started_at, finished_at, status, remote_key, size_bytes, error
FROM backup_runs
ORDER BY started_at DESC
LIMIT $1;

-- name: GetLastSucceededBackupRun :one
SELECT id, started_at, finished_at, status, remote_key, size_bytes, error
FROM backup_runs
WHERE status = 'succeeded'
ORDER BY finished_at DESC
LIMIT 1;

-- name: GetLastBackupRun :one
SELECT id, started_at, finished_at, status, remote_key, size_bytes, error
FROM backup_runs
ORDER BY started_at DESC
LIMIT 1;
