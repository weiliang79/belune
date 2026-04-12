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

-- name: SearchApplicationLogs :many
SELECT id, application_id, stream, message, recorded_at FROM application_logs
WHERE application_id = sqlc.arg('application_id')
  AND (sqlc.narg('q')::text IS NULL OR message ILIKE '%' || sqlc.narg('q')::text || '%')
  AND (sqlc.narg('stream')::text IS NULL OR stream = sqlc.narg('stream')::text)
  AND (sqlc.narg('since')::timestamptz IS NULL OR recorded_at >= sqlc.narg('since')::timestamptz)
  AND (sqlc.narg('until')::timestamptz IS NULL OR recorded_at <= sqlc.narg('until')::timestamptz)
ORDER BY recorded_at DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: GetLatestApplicationLogTime :one
SELECT MAX(recorded_at)::timestamptz FROM application_logs
WHERE application_id = $1;

-- name: DeleteOldApplicationLogs :exec
DELETE FROM application_logs WHERE recorded_at < NOW() - ($1 || ' days')::INTERVAL;
