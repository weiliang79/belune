-- name: InsertRequestLog :exec
INSERT INTO request_logs (application_id, method, path, status_code, latency_ms, hostname, request_size, response_size, client_ip, user_agent)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- name: ListRequestLogsByApplication :many
SELECT * FROM request_logs
WHERE application_id = $1
ORDER BY recorded_at DESC
LIMIT $2 OFFSET $3;

-- name: ListRequestLogs :many
SELECT * FROM request_logs
ORDER BY recorded_at DESC
LIMIT $1 OFFSET $2;

-- name: ListRequestLogsFiltered :many
SELECT * FROM request_logs
WHERE ($1::uuid IS NULL OR application_id = $1)
  AND ($2::smallint IS NULL OR status_code >= $2)
  AND ($3::smallint IS NULL OR status_code < $3)
  AND ($4::timestamptz IS NULL OR recorded_at >= $4)
  AND ($5::timestamptz IS NULL OR recorded_at <= $5)
ORDER BY recorded_at DESC
LIMIT $6 OFFSET $7;

-- name: DeleteOldRequestLogs :exec
DELETE FROM request_logs WHERE recorded_at < NOW() - ($1 || ' days')::INTERVAL;
