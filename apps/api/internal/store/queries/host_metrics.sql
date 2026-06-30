-- name: InsertHostMetric :exec
INSERT INTO host_metrics (cpu_percent, memory_used, memory_total, disk_used, disk_total, recorded_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetHostMetrics :many
SELECT id, cpu_percent, memory_used, memory_total, disk_used, disk_total, recorded_at
FROM host_metrics
WHERE recorded_at >= $1
ORDER BY recorded_at ASC;

-- name: ListHostMetricsBetween :many
SELECT id, cpu_percent, memory_used, memory_total, disk_used, disk_total, recorded_at
FROM host_metrics
WHERE recorded_at >= $1 AND recorded_at <= $2
ORDER BY recorded_at ASC;

-- name: DeleteOldHostMetrics :exec
DELETE FROM host_metrics WHERE recorded_at < $1;

-- name: CountHostMetrics :one
SELECT count(*) FROM host_metrics;
