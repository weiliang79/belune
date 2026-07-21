-- Operator-health aggregates for the dashboard stat strips. Each is optionally
-- scoped to one owner via sqlc.narg('user_id') (NULL = all, for admins).

-- name: CountApplicationHealth :one
-- Parent applications only — preview children are ephemeral and would inflate
-- the health ratio.
SELECT count(*)                                       AS total,
       count(*) FILTER (WHERE a.status = 'running')   AS running,
       count(*) FILTER (WHERE a.status = 'error')     AS errored,
       count(*) FILTER (WHERE a.status = 'stopped')   AS stopped,
       count(*) FILTER (WHERE a.status = 'unhealthy') AS unhealthy,
       count(*) FILTER (WHERE a.status = 'inactive')  AS inactive
FROM applications a
JOIN projects p ON p.id = a.project_id
WHERE a.parent_application_id IS NULL
  AND (sqlc.narg('user_id')::uuid IS NULL OR p.user_id = sqlc.narg('user_id'));

-- name: CountDatabaseHealth :one
SELECT count(*)                                       AS total,
       count(*) FILTER (WHERE db.status = 'running')  AS running,
       count(*) FILTER (WHERE db.status = 'failed')   AS errored,
       count(*) FILTER (WHERE db.status = 'stopped')  AS stopped
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

-- name: CountUnresolvedFailedDeploys :one
-- Applications that are still serving but whose most recent *resolved*
-- deployment failed.
--
-- This counts the latest outcome per application rather than every failure in a
-- time window, which is what makes the "needs attention" figure actionable: a
-- successful redeploy clears it, and a failure nobody fixed keeps showing
-- instead of silently ageing out after 7 days. In-progress deployments are
-- excluded from the "latest" pick so merely starting a retry cannot mask an
-- unfixed failure before it actually succeeds. Preview children are excluded to
-- match CountApplicationHealth — they are ephemeral and would inflate the count.
--
-- Applications already surfaced by another attention bucket ('error' and
-- 'unhealthy') are excluded so the buckets stay disjoint and sum to the headline
-- figure. A failed deploy usually also flags the application errored, and
-- counting both reported one broken app as two issues.
-- What is left here is the genuinely distinct case the deploy ordering created:
-- the previous container is still up and serving the old version, so the app is
-- not errored, but its latest deploy failed and needs looking at.
SELECT count(*) AS failed
FROM (
    SELECT DISTINCT ON (d.application_id) d.status
    FROM deployments d
    JOIN applications a ON a.id = d.application_id
    JOIN projects p ON p.id = a.project_id
    WHERE a.parent_application_id IS NULL
      AND a.status NOT IN ('error', 'unhealthy')
      AND d.status IN ('success', 'failed')
      AND (sqlc.narg('user_id')::uuid IS NULL OR p.user_id = sqlc.narg('user_id'))
    ORDER BY d.application_id, d.started_at DESC
) latest
WHERE latest.status = 'failed';

-- name: CountUnresolvedFailedScheduledBackups :one
-- Backup schedules whose most recent finished run failed, across both database
-- and application-volume backups.
--
-- These are the automated backups: they run on a cron and fail silently, which
-- is the canonical thing "needs attention" exists for — you are left believing
-- you have a backup you do not. Unlike the single global platform job, they are
-- per-config, so "unresolved" means the latest outcome per *config* failed,
-- mirroring CountUnresolvedFailedDeploys: the next successful run clears it.
--
-- Only enabled configs count; a schedule that was turned off after its last run
-- failed is not outstanding work. Runs still in progress are excluded from the
-- "latest" pick so a retry cannot mask an unfixed failure before it succeeds.
WITH latest_db AS (
    SELECT DISTINCT ON (b.backup_config_id) b.status
    FROM database_backups b
    JOIN database_backup_configs c ON c.id = b.backup_config_id
    JOIN databases db ON db.id = c.database_id
    JOIN projects p ON p.id = db.project_id
    WHERE c.enabled
      AND b.status IN ('succeeded', 'failed')
      AND (sqlc.narg('user_id')::uuid IS NULL OR p.user_id = sqlc.narg('user_id'))
    ORDER BY b.backup_config_id, b.started_at DESC
), latest_volume AS (
    SELECT DISTINCT ON (v.backup_config_id) v.status
    FROM application_volume_backups v
    JOIN application_volume_backup_configs vc ON vc.id = v.backup_config_id
    JOIN application_volumes av ON av.id = vc.application_volume_id
    JOIN applications a ON a.id = av.application_id
    JOIN projects p ON p.id = a.project_id
    WHERE vc.enabled
      AND v.status IN ('succeeded', 'failed')
      AND (sqlc.narg('user_id')::uuid IS NULL OR p.user_id = sqlc.narg('user_id'))
    ORDER BY v.backup_config_id, v.started_at DESC
)
SELECT (
    (SELECT count(*) FROM latest_db WHERE status = 'failed') +
    (SELECT count(*) FROM latest_volume WHERE status = 'failed')
)::bigint AS failed;

-- name: CountUnresolvedFailedBackup :one
-- The platform backup is a single global job (backup_runs has no per-resource
-- key), so "unresolved" is simply whether the most recent finished run failed.
-- Returns 0 or 1; the next successful run clears it. A run still in progress is
-- skipped so it cannot mask the previous failure.
SELECT count(*) AS failed
FROM (
    SELECT status
    FROM backup_runs
    WHERE status IN ('succeeded', 'failed')
    ORDER BY started_at DESC
    LIMIT 1
) latest
WHERE latest.status = 'failed';
