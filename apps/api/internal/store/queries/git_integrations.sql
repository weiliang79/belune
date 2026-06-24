-- name: CreateGitIntegration :one
INSERT INTO git_integrations (provider, base_url, account_login, config_encrypted, created_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetGitIntegration :one
SELECT * FROM git_integrations WHERE id = $1;

-- name: ListGitIntegrationsByUser :many
SELECT id, provider, base_url, account_login, created_by, created_at, updated_at
FROM git_integrations
WHERE created_by = $1
ORDER BY created_at DESC;

-- name: UpdateGitIntegrationConfig :one
UPDATE git_integrations SET config_encrypted = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteGitIntegration :exec
DELETE FROM git_integrations WHERE id = $1;

-- name: CountApplicationsUsingIntegration :one
SELECT count(*) FROM applications WHERE git_integration_id = $1;
