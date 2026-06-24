-- name: UpsertGitProviderConfig :one
INSERT INTO git_provider_configs (provider, base_url, client_id, app_id, app_slug, secret_encrypted)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (provider, base_url) DO UPDATE SET
    client_id = EXCLUDED.client_id,
    app_id = EXCLUDED.app_id,
    app_slug = EXCLUDED.app_slug,
    secret_encrypted = COALESCE(EXCLUDED.secret_encrypted, git_provider_configs.secret_encrypted),
    updated_at = NOW()
RETURNING *;

-- name: GetGitProviderConfig :one
SELECT * FROM git_provider_configs WHERE id = $1;

-- name: GetGitProviderConfigByProvider :one
SELECT * FROM git_provider_configs WHERE provider = $1 AND base_url = $2;

-- name: ListGitProviderConfigsForProvider :many
SELECT * FROM git_provider_configs WHERE provider = $1;

-- name: ListGitProviderConfigs :many
SELECT id, provider, base_url, client_id, app_id, app_slug, created_at, updated_at,
    (secret_encrypted IS NOT NULL AND length(secret_encrypted) > 0) AS has_secret
FROM git_provider_configs
ORDER BY provider, base_url;

-- name: DeleteGitProviderConfig :exec
DELETE FROM git_provider_configs WHERE id = $1;
