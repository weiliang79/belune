-- name: GetQuota :one
SELECT * FROM quotas WHERE scope = $1 AND scope_id = $2;

-- name: UpsertQuota :one
INSERT INTO quotas (scope, scope_id, max_applications, max_cpu, max_memory_mb)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (scope, scope_id) DO UPDATE SET
    max_applications = EXCLUDED.max_applications,
    max_cpu          = EXCLUDED.max_cpu,
    max_memory_mb    = EXCLUDED.max_memory_mb,
    updated_at       = NOW()
RETURNING *;

-- name: DeleteQuota :exec
DELETE FROM quotas WHERE scope = $1 AND scope_id = $2;

-- name: ListQuotas :many
SELECT * FROM quotas ORDER BY scope, created_at DESC;

-- name: CountApplicationsByProject :one
SELECT COUNT(*) FROM applications WHERE project_id = $1;

-- name: CountApplicationsByUser :one
SELECT COUNT(*)
FROM applications a
JOIN projects p ON p.id = a.project_id
WHERE p.user_id = $1;

-- name: SumApplicationResourcesByProject :one
SELECT
    COALESCE(SUM(cpu_limit), 0)::DOUBLE PRECISION AS cpu_total,
    COALESCE(SUM(memory_limit), 0)::BIGINT        AS memory_total_bytes
FROM applications
WHERE project_id = $1;

-- name: SumApplicationResourcesByUser :one
SELECT
    COALESCE(SUM(a.cpu_limit), 0)::DOUBLE PRECISION AS cpu_total,
    COALESCE(SUM(a.memory_limit), 0)::BIGINT        AS memory_total_bytes
FROM applications a
JOIN projects p ON p.id = a.project_id
WHERE p.user_id = $1;
