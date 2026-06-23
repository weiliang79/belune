-- name: ListDomainsByApplication :many
SELECT * FROM domains WHERE application_id = $1 ORDER BY created_at DESC;

-- name: CreateDomain :one
INSERT INTO domains (application_id, hostname, ssl_enabled, container_port, force_https, ssl_mode, ssl_provider, ssl_credentials_encrypted, cert_path, key_path, advanced_config)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: GetDomain :one
SELECT * FROM domains WHERE id = $1;

-- name: GetDomainByHostname :one
SELECT * FROM domains WHERE hostname = $1;

-- name: DeleteDomain :exec
DELETE FROM domains WHERE id = $1;

-- name: GetDomainOwnerUserID :one
SELECT p.user_id FROM domains d
JOIN applications a ON a.id = d.application_id
JOIN projects p ON p.id = a.project_id
WHERE d.id = $1;

-- name: UpdateDomain :one
UPDATE domains SET
    hostname = $2, ssl_enabled = $3, container_port = $4, force_https = $5,
    ssl_mode = $6, ssl_provider = $7, ssl_credentials_encrypted = $8,
    cert_path = $9, key_path = $10, advanced_config = $11
WHERE id = $1
RETURNING *;

-- name: ListDomainsByApplicationWithFeatures :many
SELECT d.*, COALESCE(
    (SELECT json_agg(json_build_object(
        'id', f.id, 'feature_type', f.feature_type,
        'config', f.config, 'enabled', f.enabled
    )) FROM domain_route_features f WHERE f.domain_id = d.id),
    '[]'::json
) AS route_features
FROM domains d
WHERE d.application_id = $1
ORDER BY d.created_at DESC;

-- name: ListProjectAppPrimaryDomain :many
-- The first-added domain (hostname + container_port) per parent application in a
-- project, for the project services table. hostname/port are NULL when an app
-- has no domain.
SELECT DISTINCT ON (a.id) a.id AS application_id, d.hostname, d.container_port
FROM applications a
LEFT JOIN domains d ON d.application_id = a.id
WHERE a.project_id = $1 AND a.parent_application_id IS NULL
ORDER BY a.id, d.created_at ASC;
