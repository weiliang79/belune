-- name: ListApplicationsByProject :many
SELECT * FROM applications
WHERE project_id = $1 AND parent_application_id IS NULL
ORDER BY created_at DESC;

-- name: GetApplication :one
SELECT * FROM applications WHERE id = $1;

-- name: GetApplicationWithProjectSlug :one
SELECT a.*, p.slug as project_slug
FROM applications a
JOIN projects p ON p.id = a.project_id
WHERE a.id = $1;

-- name: CreateApplication :one
INSERT INTO applications (project_id, name, slug, type, source_repo, source_image, dockerfile_path, build_type, cpu_limit, memory_limit, webhook_secret, git_credentials_encrypted, health_check_path, git_integration_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
RETURNING *;

-- name: UpdateApplication :one
UPDATE applications SET
    name = $2, source_repo = $3, source_image = $4, dockerfile_path = $5,
    build_type_override = $6, builder_image = $7, custom_buildpacks = $8,
    status = $9, cpu_limit = $10, memory_limit = $11, git_credentials_encrypted = $12,
    health_check_path = $13, git_integration_id = $14, updated_at = NOW()
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

-- name: UpdateApplicationPreviewConfig :one
UPDATE applications SET
    preview_branch_pattern = $2,
    preview_domain_template = $3,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: CreatePreviewApplication :one
INSERT INTO applications (
    project_id, name, slug, type,
    source_repo, source_image, dockerfile_path, build_type,
    cpu_limit, memory_limit, git_credentials_encrypted, health_check_path,
    git_integration_id, parent_application_id, branch
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
RETURNING *;

-- name: GetPreviewByParentBranch :one
SELECT * FROM applications
WHERE parent_application_id = $1 AND branch = $2;

-- name: ListPreviewsByParent :many
SELECT * FROM applications
WHERE parent_application_id = $1
ORDER BY last_activity_at DESC;

-- name: ListStalePreviews :many
SELECT * FROM applications
WHERE parent_application_id IS NOT NULL
  AND last_activity_at < $1;

-- name: ListAllApplicationsWithProjectSlug :many
SELECT a.*, p.slug as project_slug
FROM applications a
JOIN projects p ON p.id = a.project_id;

-- name: ListStalePreviewsWithProjectSlug :many
SELECT a.*, p.slug as project_slug
FROM applications a
JOIN projects p ON p.id = a.project_id
WHERE a.parent_application_id IS NOT NULL
  AND a.last_activity_at < $1;

-- name: TouchApplicationActivity :exec
UPDATE applications SET last_activity_at = NOW()
WHERE id = $1;

-- name: GetDeploymentOwnerInfo :one
SELECT u.id AS user_id, u.email, u.first_name, a.name AS app_name, p.name AS project_name
FROM deployments d
JOIN applications a ON a.id = d.application_id
JOIN projects p ON p.id = a.project_id
JOIN users u ON u.id = p.user_id
WHERE d.id = $1;

-- name: GetProjectOwnerInfo :one
SELECT u.id AS user_id, u.email, u.first_name, p.name AS project_name
FROM projects p
JOIN users u ON u.id = p.user_id
WHERE p.id = $1;

-- name: UpdateApplicationSource :exec
UPDATE applications SET source_kind = $2, source_ref = $3, updated_at = NOW()
WHERE id = $1;

-- name: UpdateApplicationRuntime :exec
UPDATE applications SET readonly_rootfs = $2, container_caps = $3, updated_at = NOW()
WHERE id = $1;

-- name: SetApplicationContainerPort :exec
UPDATE applications SET container_port = $2, updated_at = NOW()
WHERE id = $1;

-- name: SetApplicationHealthTuning :exec
UPDATE applications
SET health_check_timeout_seconds = $2, health_check_expect_status = $3, updated_at = NOW()
WHERE id = $1;
