-- name: CreateGitCredential :one
INSERT INTO git_credentials (name, provider, token_encrypted, username, created_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetGitCredential :one
SELECT * FROM git_credentials WHERE id = $1;

-- name: ListGitCredentials :many
SELECT id, name, provider, username, created_by, created_at, updated_at
FROM git_credentials
ORDER BY created_at DESC;

-- name: UpdateGitCredential :one
UPDATE git_credentials
SET name = $2, provider = $3, token_encrypted = $4, username = $5, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteGitCredential :exec
DELETE FROM git_credentials WHERE id = $1;

-- name: CountGitCredentialsUsingCredential :one
SELECT count(*) FROM applications WHERE git_credential_id = $1;
