-- name: InsertContainerLog :exec
INSERT INTO container_logs (source_type, source_id, level, stream, message, recorded_at)
VALUES ($1, $2, $3, $4, $5, COALESCE(sqlc.narg('recorded_at')::timestamptz, NOW()));

-- name: SearchContainerLogs :many
SELECT id, source_type, source_id, level, stream, message, recorded_at FROM container_logs
WHERE source_type = sqlc.arg('source_type')
  AND source_id = sqlc.arg('source_id')
  AND (sqlc.narg('level')::text IS NULL OR level = sqlc.narg('level')::text)
  AND (sqlc.narg('stream')::text IS NULL OR stream = sqlc.narg('stream')::text)
  AND (sqlc.narg('q')::text IS NULL OR message ILIKE '%' || sqlc.narg('q')::text || '%')
  AND (sqlc.narg('since')::timestamptz IS NULL OR recorded_at >= sqlc.narg('since')::timestamptz)
  AND (sqlc.narg('until')::timestamptz IS NULL OR recorded_at <= sqlc.narg('until')::timestamptz)
ORDER BY recorded_at DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: GetLatestContainerLogTime :one
SELECT MAX(recorded_at)::timestamptz FROM container_logs
WHERE source_type = $1 AND source_id = $2;

-- name: DeleteOldContainerLogs :exec
DELETE FROM container_logs WHERE recorded_at < NOW() - ($1 || ' days')::INTERVAL;
