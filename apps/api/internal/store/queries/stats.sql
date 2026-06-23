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
-- median_build_ms is the median build duration (build_ended_at - build_started_at)
-- over completed builds in the window; 0 when there are none.
SELECT count(*)                                       AS total,
       count(*) FILTER (WHERE d.status = 'success')   AS succeeded,
       count(*) FILTER (WHERE d.status = 'failed')    AS failed,
       COALESCE(
         percentile_cont(0.5) WITHIN GROUP (
           ORDER BY EXTRACT(EPOCH FROM (d.build_ended_at - d.build_started_at)) * 1000
         ) FILTER (WHERE d.build_started_at IS NOT NULL AND d.build_ended_at IS NOT NULL),
         0
       )::float8                                      AS median_build_ms
FROM deployments d
JOIN applications a ON a.id = d.application_id
JOIN projects p ON p.id = a.project_id
WHERE d.started_at >= now() - interval '7 days'
  AND (sqlc.narg('user_id')::uuid IS NULL OR p.user_id = sqlc.narg('user_id'));

-- name: CountFailedBackups7d :one
SELECT count(*) AS failed
FROM backup_runs
WHERE status = 'failed' AND started_at >= now() - interval '7 days';
