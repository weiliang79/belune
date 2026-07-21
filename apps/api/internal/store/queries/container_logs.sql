-- name: InsertContainerLog :exec
INSERT INTO container_logs (source_type, source_id, level, stream, message, recorded_at, deployment_id)
VALUES ($1, $2, $3, $4, $5, COALESCE(sqlc.narg('recorded_at')::timestamptz, NOW()), sqlc.narg('deployment_id'));

-- name: BulkInsertContainerLogs :copyfrom
-- Bulk path for the log collector, which flushes batches of lines. Inserting
-- them one statement at a time cost a round trip per line and dominated the
-- write cost of the busiest table in the system.
--
-- CopyFrom takes plain column values, so recorded_at has no COALESCE fallback
-- here: the caller always supplies it, using the timestamp Docker reported for
-- the line and its own clock only when that is missing. That is closer to when
-- the line was actually observed than evaluating NOW() at flush time anyway.
INSERT INTO container_logs (source_type, source_id, level, stream, message, recorded_at, deployment_id)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: SearchContainerLogs :many
SELECT id, source_type, source_id, level, stream, message, recorded_at, deployment_id FROM container_logs
WHERE source_type = sqlc.arg('source_type')
  AND source_id = sqlc.arg('source_id')
  AND (sqlc.narg('level')::text IS NULL OR level = sqlc.narg('level')::text)
  AND (sqlc.narg('stream')::text IS NULL OR stream = sqlc.narg('stream')::text)
  AND (sqlc.narg('q')::text IS NULL OR message ILIKE '%' || sqlc.narg('q')::text || '%')
  AND (sqlc.narg('since')::timestamptz IS NULL OR recorded_at >= sqlc.narg('since')::timestamptz)
  AND (sqlc.narg('until')::timestamptz IS NULL OR recorded_at <= sqlc.narg('until')::timestamptz)
  -- Session filter: a specific deployment, the unassigned (NULL) bucket, or no
  -- filter at all. 'unassigned' lets the caller ask for the "earlier" bucket,
  -- which a nullable deployment_id arg alone could not distinguish from "any".
  AND (CASE
    WHEN sqlc.narg('deployment_id')::uuid IS NOT NULL THEN deployment_id = sqlc.narg('deployment_id')::uuid
    WHEN sqlc.narg('unassigned')::boolean THEN deployment_id IS NULL
    ELSE TRUE
  END)
ORDER BY recorded_at DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: ListContainerLogSessions :many
-- One row per deployment (image generation) that has produced log lines for this
-- source, most recent first, enriched with the deployment's own metadata. The
-- LEFT JOIN keeps the NULL "earlier" bucket (and database/system logs, which
-- have no deployment) in the result.
--
-- Bounded: an application accumulates a session per deploy for its whole life,
-- and every row here becomes an entry in the viewer's session picker. The cap
-- keeps both the response and that dropdown finite. Truncation is safe — the
-- viewer falls back to a short id for any session it cannot label, and "All
-- deployments" still reads every line regardless of what the picker lists.
SELECT cl.deployment_id,
       MIN(cl.recorded_at)::timestamptz AS first_at,
       MAX(cl.recorded_at)::timestamptz AS last_at,
       COUNT(*)                         AS line_count,
       d.triggered_by,
       d.status,
       d.commit_sha,
       d.started_at
FROM container_logs cl
LEFT JOIN deployments d ON d.id = cl.deployment_id
WHERE cl.source_type = $1 AND cl.source_id = $2
GROUP BY cl.deployment_id, d.triggered_by, d.status, d.commit_sha, d.started_at
ORDER BY MAX(cl.recorded_at) DESC
LIMIT sqlc.arg('limit');

-- name: GetLatestContainerLogTime :one
SELECT MAX(recorded_at)::timestamptz FROM container_logs
WHERE source_type = $1 AND source_id = $2;

-- name: DeleteOldContainerLogs :exec
DELETE FROM container_logs WHERE recorded_at < NOW() - ($1 || ' days')::INTERVAL;
