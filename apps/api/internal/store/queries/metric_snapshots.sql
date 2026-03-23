-- name: InsertMetricSnapshot :exec
INSERT INTO metric_snapshots (
    application_id, granularity,
    host_cpu_percent, host_memory_used, host_memory_total,
    host_disk_used, host_disk_total,
    cpu_percent, memory_usage, memory_limit,
    network_rx_bytes, network_tx_bytes,
    recorded_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13);

-- name: GetHostMetrics :many
SELECT id, granularity,
    host_cpu_percent, host_memory_used, host_memory_total,
    host_disk_used, host_disk_total,
    recorded_at
FROM metric_snapshots
WHERE application_id IS NULL
  AND granularity = $1
  AND recorded_at >= $2
ORDER BY recorded_at ASC;

-- name: GetApplicationMetrics :many
SELECT id, granularity,
    cpu_percent, memory_usage, memory_limit,
    network_rx_bytes, network_tx_bytes,
    recorded_at
FROM metric_snapshots
WHERE application_id = $1
  AND granularity = $2
  AND recorded_at >= $3
ORDER BY recorded_at ASC;

-- name: DeleteOldMetricSnapshots :exec
DELETE FROM metric_snapshots
WHERE granularity = $1 AND recorded_at < $2;

-- name: DownsampleMetrics5m :exec
INSERT INTO metric_snapshots (
    application_id, granularity,
    host_cpu_percent, host_memory_used, host_memory_total,
    host_disk_used, host_disk_total,
    cpu_percent, memory_usage, memory_limit,
    network_rx_bytes, network_tx_bytes,
    recorded_at
)
SELECT
    s.application_id, '5m',
    AVG(s.host_cpu_percent), AVG(s.host_memory_used), MAX(s.host_memory_total),
    AVG(s.host_disk_used), MAX(s.host_disk_total),
    AVG(s.cpu_percent), AVG(s.memory_usage), MAX(s.memory_limit),
    AVG(s.network_rx_bytes), AVG(s.network_tx_bytes),
    date_trunc('hour', s.recorded_at) + INTERVAL '5 min' * FLOOR(EXTRACT(MINUTE FROM s.recorded_at) / 5)
FROM metric_snapshots s
WHERE s.granularity = '1m'
  AND s.recorded_at < $1
  AND s.recorded_at >= $2
GROUP BY s.application_id, date_trunc('hour', s.recorded_at) + INTERVAL '5 min' * FLOOR(EXTRACT(MINUTE FROM s.recorded_at) / 5);

-- name: DownsampleMetrics1h :exec
INSERT INTO metric_snapshots (
    application_id, granularity,
    host_cpu_percent, host_memory_used, host_memory_total,
    host_disk_used, host_disk_total,
    cpu_percent, memory_usage, memory_limit,
    network_rx_bytes, network_tx_bytes,
    recorded_at
)
SELECT
    s.application_id, '1h',
    AVG(s.host_cpu_percent), AVG(s.host_memory_used), MAX(s.host_memory_total),
    AVG(s.host_disk_used), MAX(s.host_disk_total),
    AVG(s.cpu_percent), AVG(s.memory_usage), MAX(s.memory_limit),
    AVG(s.network_rx_bytes), AVG(s.network_tx_bytes),
    date_trunc('hour', s.recorded_at)
FROM metric_snapshots s
WHERE s.granularity = '5m'
  AND s.recorded_at < $1
  AND s.recorded_at >= $2
GROUP BY s.application_id, date_trunc('hour', s.recorded_at);
