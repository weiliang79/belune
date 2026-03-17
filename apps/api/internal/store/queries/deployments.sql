-- name: ListDeploymentsByService :many
SELECT * FROM deployments WHERE service_id = $1 ORDER BY started_at DESC;

-- name: GetDeployment :one
SELECT * FROM deployments WHERE id = $1;

-- name: CreateDeployment :one
INSERT INTO deployments (service_id, status, triggered_by, commit_sha)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdateDeploymentStatus :one
UPDATE deployments SET status = $2, error_message = $3, finished_at = NOW()
WHERE id = $1
RETURNING *;
