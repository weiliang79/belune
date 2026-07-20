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
-- branch: the ref to build. NULL means the repository's default ref, which is
-- what every application did before branch selection existed.
-- auto_deploy_branch is kept in lockstep with it: one user-facing "Branch"
-- decides both what we build and which pushes deploy, so the two cannot drift.
INSERT INTO applications (project_id, name, slug, type, source_repo, source_image, dockerfile_path, build_type, cpu_limit, memory_limit, webhook_secret_encrypted, git_credentials_encrypted, health_check_path, git_integration_id, branch, auto_deploy_branch)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
RETURNING *;

-- name: UpdateApplication :one
UPDATE applications SET
    name = $2, source_repo = $3, source_image = $4, dockerfile_path = $5,
    build_type_override = $6, builder_image = $7, custom_buildpacks = $8,
    status = $9, cpu_limit = $10, memory_limit = $11, git_credentials_encrypted = $12,
    health_check_path = $13, git_integration_id = $14,
    branch = $15, auto_deploy_branch = $16, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateApplicationStatus :one
UPDATE applications SET status = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteApplication :exec
DELETE FROM applications WHERE id = $1;

-- name: ListApplicationsBySourceRepo :many
-- Either column counts as "has a secret": a row may still be un-backfilled.
SELECT * FROM applications
WHERE source_repo = $1
  AND (webhook_secret_encrypted IS NOT NULL OR webhook_secret IS NOT NULL);

-- name: UpdateApplicationWebhook :one
-- Writing the secret always clears the legacy plaintext column, so a row that
-- has been touched since the encryption migration never carries both.
UPDATE applications SET
    webhook_secret_encrypted = $2,
    webhook_secret = NULL,
    auto_deploy_branch = $3,
    updated_at = NOW()
WHERE id = $1 RETURNING *;

-- name: BackfillWebhookSecret :exec
-- One-time move of a plaintext secret into the encrypted column.
UPDATE applications SET webhook_secret_encrypted = $2, webhook_secret = NULL
WHERE id = $1;

-- name: ListApplicationsWithPlaintextWebhookSecret :many
SELECT * FROM applications
WHERE webhook_secret IS NOT NULL AND webhook_secret <> '';

-- name: SetApplicationDeployHook :one
UPDATE applications SET
    deploy_hook_token_hash = $2,
    deploy_hook_token_encrypted = $3,
    updated_at = NOW()
WHERE id = $1 RETURNING *;

-- name: ClearApplicationDeployHook :one
UPDATE applications SET
    deploy_hook_token_hash = NULL,
    deploy_hook_token_encrypted = NULL,
    updated_at = NOW()
WHERE id = $1 RETURNING *;

-- name: GetApplicationByDeployHookToken :one
SELECT * FROM applications WHERE deploy_hook_token_hash = $1;

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

-- name: TouchApplicationConfigChanged :exec
-- Marks the application as having saved config that the running container does
-- not yet reflect. Idempotent in effect: re-stamping an already-set marker just
-- moves it later, which only makes the clearing condition stricter.
UPDATE applications SET config_changed_at = NOW()
WHERE id = $1;

-- name: TouchApplicationSourceChanged :exec
-- Only config_changed_at's stronger sibling is stamped: the indicator reports
-- the stronger of the two, so a source change need not also set the config
-- marker, and doing so would leave a stale "reload to apply" behind after the
-- deploy cleared only the source marker.
UPDATE applications SET source_changed_at = NOW()
WHERE id = $1;

-- name: ClearApplicationConfigChanged :exec
-- The skip-build path (reload, rollback): the container is recreated from the
-- image already present, so config is applied but the source is not.
-- Only clears a marker that predates the deployment, so an edit made while the
-- deploy was running keeps its marker rather than being silently swallowed.
UPDATE applications SET config_changed_at = NULL
WHERE applications.id = $1
  AND config_changed_at IS NOT NULL
  AND config_changed_at <= (SELECT started_at FROM deployments WHERE deployments.id = $2);

-- name: ClearApplicationSourceChanged :exec
-- The build/pull path additionally clears the source marker; the worker calls
-- this alongside ClearApplicationConfigChanged, since a new image applies both.
-- Same predates-the-deployment guard.
UPDATE applications SET source_changed_at = NULL
WHERE applications.id = $1
  AND source_changed_at IS NOT NULL
  AND source_changed_at <= (SELECT started_at FROM deployments WHERE deployments.id = $2);

-- name: TouchApplicationDeployed :exec
UPDATE applications SET last_deployed_at = NOW()
WHERE id = $1;
