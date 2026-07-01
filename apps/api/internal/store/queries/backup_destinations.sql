-- name: CreateBackupDestination :one
INSERT INTO backup_destinations (
    project_id, name, provider, endpoint, region, bucket, prefix, use_ssl, credentials_encrypted
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: UpdateBackupDestination :one
UPDATE backup_destinations
SET name                  = $2,
    provider              = $3,
    endpoint              = $4,
    region                = $5,
    bucket                = $6,
    prefix                = $7,
    use_ssl               = $8,
    -- Keep the existing secret when no new one is supplied (NULL).
    credentials_encrypted = COALESCE($9, credentials_encrypted),
    updated_at            = NOW()
WHERE id = $1
RETURNING *;

-- name: GetBackupDestination :one
SELECT * FROM backup_destinations WHERE id = $1;

-- name: ListBackupDestinationsByProject :many
SELECT id, project_id, name, provider, endpoint, region, bucket, prefix, use_ssl, created_at, updated_at
FROM backup_destinations
WHERE project_id = $1
ORDER BY name;

-- name: DeleteBackupDestination :exec
DELETE FROM backup_destinations WHERE id = $1;
