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
-- resource_name resolves resource_id to the app/project/db name. Comparing
-- <table>.id::text = resource_id avoids casting the (possibly non-uuid)
-- resource_id to uuid, which would error for ids like 'settings'.
SELECT al.id, al.user_id, al.action, al.resource_type, al.resource_id, al.details, al.ip_address, al.created_at,
       u.email AS user_email, u.username AS user_username,
       -- Empty-string fallback so sqlc's non-null string scan never hits a NULL
       -- (rows whose resource isn't an app/project/db resolve to no name).
       COALESCE(app.name, proj.name, db.name, '')::text AS resource_name
FROM audit_logs al
LEFT JOIN users u ON u.id = al.user_id
LEFT JOIN applications app ON al.resource_type = 'application' AND app.id::text = al.resource_id
LEFT JOIN projects proj ON al.resource_type = 'project' AND proj.id::text = al.resource_id
LEFT JOIN databases db ON al.resource_type = 'database' AND db.id::text = al.resource_id
WHERE (sqlc.narg('user_id')::uuid IS NULL OR al.user_id = sqlc.narg('user_id'))
  AND (sqlc.narg('action')::text IS NULL OR al.action = sqlc.narg('action'))
  AND (sqlc.narg('resource_type')::text IS NULL OR al.resource_type = sqlc.narg('resource_type'))
  AND (sqlc.narg('resource_id')::text IS NULL OR al.resource_id = sqlc.narg('resource_id'))
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
  AND (sqlc.narg('resource_id')::text IS NULL OR al.resource_id = sqlc.narg('resource_id'))
  AND (sqlc.narg('from')::timestamptz IS NULL OR al.created_at >= sqlc.narg('from'))
  AND (sqlc.narg('to')::timestamptz IS NULL OR al.created_at <= sqlc.narg('to'));

-- name: ListDistinctAuditActions :many
SELECT DISTINCT action FROM audit_logs ORDER BY action ASC;

-- name: DeleteOldAuditLogs :exec
DELETE FROM audit_logs WHERE created_at < NOW() - ($1 || ' days')::INTERVAL;
