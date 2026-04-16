-- name: ListDeploymentsByApplication :many
SELECT * FROM deployments WHERE application_id = $1 ORDER BY started_at DESC;

-- name: GetDeployment :one
SELECT * FROM deployments WHERE id = $1;

-- name: CreateDeployment :one
INSERT INTO deployments (application_id, status, triggered_by, commit_sha, idempotency_key)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: FindRecentDeploymentByIdempotencyKey :one
SELECT * FROM deployments
WHERE application_id = $1
  AND idempotency_key = $2
  AND started_at > NOW() - make_interval(secs => sqlc.arg(window_seconds)::int)
ORDER BY started_at DESC
LIMIT 1;

-- name: UpdateDeploymentStatus :one
UPDATE deployments SET status = $2, error_message = $3, finished_at = NOW()
WHERE id = $1
RETURNING *;

-- name: SetDeploymentBuildStarted :exec
UPDATE deployments SET build_started_at = NOW() WHERE id = $1;

-- name: SetDeploymentBuildEnded :exec
UPDATE deployments SET build_ended_at = NOW() WHERE id = $1;

-- name: SetDeploymentDeployStarted :exec
UPDATE deployments SET deploy_started_at = NOW() WHERE id = $1;

-- name: UpdateDeploymentBuildLogs :exec
UPDATE deployments SET build_logs = $2 WHERE id = $1;

-- name: UpdateDeploymentImageTag :exec
UPDATE deployments SET image_tag = $2 WHERE id = $1;

-- name: ListOldDeployments :many
SELECT * FROM deployments WHERE application_id = $1 ORDER BY started_at DESC OFFSET $2;

-- name: DeleteDeployment :exec
DELETE FROM deployments WHERE id = $1;

-- name: CountDeployments :one
SELECT count(*) FROM deployments;

-- name: ListGlobalDeployments :many
SELECT d.id, d.application_id, d.status, d.triggered_by, d.commit_sha, d.build_logs, d.error_message, d.started_at, d.finished_at, d.image_tag,
       a.name AS application_name, a.slug AS application_slug,
       p.id AS project_id, p.name AS project_name
FROM deployments d
JOIN applications a ON a.id = d.application_id
JOIN projects p ON p.id = a.project_id
ORDER BY d.started_at DESC
LIMIT $1 OFFSET $2;

-- name: ListGlobalDeploymentsFiltered :many
SELECT d.id, d.application_id, d.status, d.triggered_by, d.commit_sha, d.build_logs, d.error_message, d.started_at, d.finished_at, d.image_tag,
       a.name AS application_name, a.slug AS application_slug,
       p.id AS project_id, p.name AS project_name
FROM deployments d
JOIN applications a ON a.id = d.application_id
JOIN projects p ON p.id = a.project_id
WHERE (sqlc.narg('project_id')::uuid IS NULL OR p.id = sqlc.narg('project_id'))
  AND (sqlc.narg('application_id')::uuid IS NULL OR d.application_id = sqlc.narg('application_id'))
  AND (sqlc.narg('status')::text IS NULL OR d.status = sqlc.narg('status'))
  AND (sqlc.narg('from')::timestamptz IS NULL OR d.started_at >= sqlc.narg('from'))
  AND (sqlc.narg('to')::timestamptz IS NULL OR d.started_at <= sqlc.narg('to'))
  AND (sqlc.narg('user_id')::uuid IS NULL OR p.user_id = sqlc.narg('user_id'))
ORDER BY d.started_at DESC
LIMIT $1 OFFSET $2;

-- name: ListUserDeployments :many
SELECT d.id, d.application_id, d.status, d.triggered_by, d.commit_sha, d.build_logs, d.error_message, d.started_at, d.finished_at, d.image_tag,
       a.name AS application_name, a.slug AS application_slug,
       p.id AS project_id, p.name AS project_name
FROM deployments d
JOIN applications a ON a.id = d.application_id
JOIN projects p ON p.id = a.project_id
WHERE p.user_id = $1
ORDER BY d.started_at DESC
LIMIT $2 OFFSET $3;
