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

-- name: DeleteOldRequestLogs :exec
DELETE FROM request_logs WHERE recorded_at < NOW() - ($1 || ' days')::INTERVAL;
