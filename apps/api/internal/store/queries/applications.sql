-- name: ListApplicationsByProject :many
SELECT * FROM applications WHERE project_id = $1 ORDER BY created_at DESC;

-- name: GetApplication :one
SELECT * FROM applications WHERE id = $1;

-- name: GetApplicationWithProjectSlug :one
SELECT a.*, p.slug as project_slug
FROM applications a
JOIN projects p ON p.id = a.project_id
WHERE a.id = $1;

-- name: CreateApplication :one
INSERT INTO applications (project_id, name, slug, type, source_repo, source_image, dockerfile_path, build_type, cpu_limit, memory_limit, webhook_secret, git_credentials_encrypted, health_check_path, port, git_credential_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
RETURNING *;

-- name: UpdateApplication :one
UPDATE applications SET
    name = $2, source_repo = $3, source_image = $4, dockerfile_path = $5,
    build_type_override = $6, builder_image = $7, custom_buildpacks = $8,
    status = $9, cpu_limit = $10, memory_limit = $11, git_credentials_encrypted = $12,
    health_check_path = $13, port = $14, git_credential_id = $15, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateApplicationStatus :one
UPDATE applications SET status = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteApplication :exec
DELETE FROM applications WHERE id = $1;

-- name: ListApplicationsBySourceRepo :many
SELECT * FROM applications WHERE source_repo = $1 AND webhook_secret IS NOT NULL;

-- name: UpdateApplicationWebhook :one
UPDATE applications SET webhook_secret = $2, auto_deploy_branch = $3, updated_at = NOW()
WHERE id = $1 RETURNING *;

-- name: ListAllApplications :many
SELECT * FROM applications;

-- name: UpdateApplicationSlug :exec
UPDATE applications SET slug = $2 WHERE id = $1;

-- name: CountApplications :one
SELECT count(*) FROM applications;

-- name: GetApplicationOwnerUserID :one
SELECT p.user_id FROM applications a
JOIN projects p ON p.id = a.project_id
WHERE a.id = $1;
