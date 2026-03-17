-- name: ListEnvVarsByService :many
SELECT * FROM env_vars WHERE service_id = $1 ORDER BY key;

-- name: UpsertEnvVar :one
INSERT INTO env_vars (service_id, key, value_encrypted, is_secret)
VALUES ($1, $2, $3, $4)
ON CONFLICT (service_id, key) DO UPDATE SET value_encrypted = $3, is_secret = $4, updated_at = NOW()
RETURNING *;

-- name: DeleteEnvVar :exec
DELETE FROM env_vars WHERE id = $1;

-- name: DeleteEnvVarsByService :exec
DELETE FROM env_vars WHERE service_id = $1;
