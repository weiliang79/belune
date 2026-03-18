-- name: ListServicesByProject :many
SELECT * FROM services WHERE project_id = $1 ORDER BY created_at DESC;

-- name: GetService :one
SELECT * FROM services WHERE id = $1;

-- name: CreateService :one
INSERT INTO services (project_id, name, type, source_repo, source_image, dockerfile_path, build_type)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: UpdateService :one
UPDATE services SET
    name = $2, source_repo = $3, source_image = $4, dockerfile_path = $5,
    build_type_override = $6, builder_image = $7, custom_buildpacks = $8,
    status = $9, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateServiceStatus :one
UPDATE services SET status = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteService :exec
DELETE FROM services WHERE id = $1;

-- name: ListServicesBySourceRepo :many
SELECT * FROM services WHERE source_repo = $1 AND webhook_secret IS NOT NULL;

-- name: UpdateServiceWebhook :one
UPDATE services SET webhook_secret = $2, auto_deploy_branch = $3, updated_at = NOW()
WHERE id = $1 RETURNING *;
