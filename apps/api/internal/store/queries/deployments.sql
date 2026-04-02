-- name: ListDeploymentsByApplication :many
SELECT * FROM deployments WHERE application_id = $1 ORDER BY started_at DESC;

-- name: GetDeployment :one
SELECT * FROM deployments WHERE id = $1;

-- name: CreateDeployment :one
INSERT INTO deployments (application_id, status, triggered_by, commit_sha)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdateDeploymentStatus :one
UPDATE deployments SET status = $2, error_message = $3, finished_at = NOW()
WHERE id = $1
RETURNING *;

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
