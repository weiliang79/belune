-- name: InsertBackupRun :one
INSERT INTO backup_runs (trigger)
VALUES ($1)
RETURNING id, started_at, finished_at, status, remote_key, size_bytes, error, log, trigger;

-- name: UpdateBackupRun :exec
UPDATE backup_runs
SET finished_at = $2,
    status      = $3,
    remote_key  = $4,
    size_bytes  = $5,
    error       = $6,
    log         = $7
WHERE id = $1;

-- name: ListBackupRuns :many
SELECT id, started_at, finished_at, status, remote_key, size_bytes, error, log, trigger
FROM backup_runs
ORDER BY started_at DESC
LIMIT $1 OFFSET $2;

-- name: GetLastSucceededBackupRun :one
SELECT id, started_at, finished_at, status, remote_key, size_bytes, error, log, trigger
FROM backup_runs
WHERE status = 'succeeded'
ORDER BY finished_at DESC
LIMIT 1;

-- name: GetLastBackupRun :one
SELECT id, started_at, finished_at, status, remote_key, size_bytes, error, log, trigger
FROM backup_runs
ORDER BY started_at DESC
LIMIT 1;
