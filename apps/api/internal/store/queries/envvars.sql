-- name: ListEnvVarsByApplication :many
SELECT * FROM env_vars WHERE application_id = $1 ORDER BY key;

-- name: UpsertEnvVar :one
INSERT INTO env_vars (application_id, key, value_encrypted, is_secret)
VALUES ($1, $2, $3, $4)
ON CONFLICT (application_id, key) DO UPDATE SET value_encrypted = $3, is_secret = $4, updated_at = NOW()
RETURNING *;

-- name: DeleteEnvVar :exec
DELETE FROM env_vars WHERE id = $1;

-- name: DeleteEnvVarsByApplication :exec
DELETE FROM env_vars WHERE application_id = $1;

-- name: DeleteEnvVarsNotIn :exec
-- Removes the variables the caller did not submit, which is how a delete in the
-- UI reaches the database: the update endpoint upserts what it was given, and
-- anything absent from that set is gone. Passing an empty array deletes them
-- all, which is the correct reading of "the app now has no variables".
DELETE FROM env_vars
WHERE application_id = $1
  AND key <> ALL(sqlc.arg('keys')::text[]);
