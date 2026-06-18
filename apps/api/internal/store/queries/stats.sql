-- Operator-health aggregates for the dashboard stat strips. Each is optionally
-- scoped to one owner via sqlc.narg('user_id') (NULL = all, for admins).

-- name: CountApplicationHealth :one
-- Parent applications only — preview children are ephemeral and would inflate
-- the health ratio.
SELECT count(*)                                      AS total,
       count(*) FILTER (WHERE a.status = 'running')  AS running,
       count(*) FILTER (WHERE a.status = 'error')    AS errored
FROM applications a
JOIN projects p ON p.id = a.project_id
WHERE a.parent_application_id IS NULL
  AND (sqlc.narg('user_id')::uuid IS NULL OR p.user_id = sqlc.narg('user_id'));

-- name: CountDatabaseHealth :one
SELECT count(*)                                       AS total,
       count(*) FILTER (WHERE db.status = 'running')  AS running,
       count(*) FILTER (WHERE db.status = 'failed')   AS errored
FROM databases db
JOIN projects p ON p.id = db.project_id
WHERE (sqlc.narg('user_id')::uuid IS NULL OR p.user_id = sqlc.narg('user_id'));

-- name: CountDeployments7d :one
SELECT count(*)                                       AS total,
       count(*) FILTER (WHERE d.status = 'success')   AS succeeded,
       count(*) FILTER (WHERE d.status = 'failed')    AS failed
FROM deployments d
JOIN applications a ON a.id = d.application_id
JOIN projects p ON p.id = a.project_id
WHERE d.started_at >= now() - interval '7 days'
  AND (sqlc.narg('user_id')::uuid IS NULL OR p.user_id = sqlc.narg('user_id'));

-- name: CountFailedBackups7d :one
SELECT count(*) AS failed
FROM backup_runs
WHERE status = 'failed' AND started_at >= now() - interval '7 days';
