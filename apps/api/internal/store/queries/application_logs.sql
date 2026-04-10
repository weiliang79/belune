-- name: InsertApplicationLog :exec
INSERT INTO application_logs (application_id, stream, message)
VALUES ($1, $2, $3);

-- name: InsertApplicationLogsBatch :copyfrom
INSERT INTO application_logs (application_id, stream, message)
VALUES ($1, $2, $3);

-- name: ListApplicationLogsByApplication :many
SELECT * FROM application_logs
WHERE application_id = $1
ORDER BY recorded_at DESC
LIMIT $2 OFFSET $3;

-- name: GetLatestApplicationLogTime :one
SELECT MAX(recorded_at)::timestamptz FROM application_logs
WHERE application_id = $1;

-- name: DeleteOldApplicationLogs :exec
DELETE FROM application_logs WHERE recorded_at < NOW() - ($1 || ' days')::INTERVAL;
