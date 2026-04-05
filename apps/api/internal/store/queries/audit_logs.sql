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
