-- name: CreateAuditLog :exec
INSERT INTO audit_logs (user_id, action, resource_type, resource_id, details, ip_address)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: ListAuditLogs :many
SELECT al.*, u.email AS user_email, u.username AS user_username
FROM audit_logs al
LEFT JOIN users u ON u.id = al.user_id
ORDER BY al.created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountAuditLogs :one
SELECT count(*) FROM audit_logs;

-- name: ListAuditLogsFiltered :many
SELECT al.id, al.user_id, al.action, al.resource_type, al.resource_id, al.details, al.ip_address, al.created_at,
       u.email AS user_email, u.username AS user_username
FROM audit_logs al
LEFT JOIN users u ON u.id = al.user_id
WHERE (sqlc.narg('user_id')::uuid IS NULL OR al.user_id = sqlc.narg('user_id'))
  AND (sqlc.narg('action')::text IS NULL OR al.action = sqlc.narg('action'))
  AND (sqlc.narg('resource_type')::text IS NULL OR al.resource_type = sqlc.narg('resource_type'))
  AND (sqlc.narg('from')::timestamptz IS NULL OR al.created_at >= sqlc.narg('from'))
  AND (sqlc.narg('to')::timestamptz IS NULL OR al.created_at <= sqlc.narg('to'))
ORDER BY al.created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountAuditLogsFiltered :one
SELECT count(*)
FROM audit_logs al
WHERE (sqlc.narg('user_id')::uuid IS NULL OR al.user_id = sqlc.narg('user_id'))
  AND (sqlc.narg('action')::text IS NULL OR al.action = sqlc.narg('action'))
  AND (sqlc.narg('resource_type')::text IS NULL OR al.resource_type = sqlc.narg('resource_type'))
  AND (sqlc.narg('from')::timestamptz IS NULL OR al.created_at >= sqlc.narg('from'))
  AND (sqlc.narg('to')::timestamptz IS NULL OR al.created_at <= sqlc.narg('to'));
