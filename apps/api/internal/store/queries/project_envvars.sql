-- name: ListProjectEnvVars :many
SELECT * FROM project_env_vars WHERE project_id = $1 ORDER BY key;

-- name: UpsertProjectEnvVar :one
INSERT INTO project_env_vars (project_id, key, value_encrypted, is_secret)
VALUES ($1, $2, $3, $4)
ON CONFLICT (project_id, key) DO UPDATE SET value_encrypted = $3, is_secret = $4, updated_at = NOW()
RETURNING *;

-- name: DeleteProjectEnvVar :exec
DELETE FROM project_env_vars WHERE id = $1;

-- name: DeleteProjectEnvVarsByProject :exec
DELETE FROM project_env_vars WHERE project_id = $1;

-- name: DeleteProjectEnvVarsNotIn :exec
-- Project counterpart of DeleteEnvVarsNotIn: removes what the caller did not
-- submit, so the stored set matches the submitted one exactly.
DELETE FROM project_env_vars
WHERE project_id = $1
  AND key <> ALL(sqlc.arg('keys')::text[]);
