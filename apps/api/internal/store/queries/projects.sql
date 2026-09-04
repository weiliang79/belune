-- name: ListProjectsByUser :many
-- Owned or shared: a shared project is visible to every Member, not only its owner.
SELECT p.*, (
    SELECT max(d.started_at)
    FROM deployments d
    JOIN applications a ON a.id = d.application_id
    WHERE a.project_id = p.id
) AS last_deployed_at
FROM projects p
WHERE p.user_id = $1 OR p.shared
ORDER BY p.created_at DESC;

-- name: GetProject :one
SELECT * FROM projects WHERE id = $1;

-- name: CreateProject :one
INSERT INTO projects (name, slug, user_id, server_id)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdateProject :one
UPDATE projects SET name = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteProject :exec
DELETE FROM projects WHERE id = $1;

-- name: CountProjects :one
SELECT count(*) FROM projects;

-- name: ListAllProjects :many
SELECT p.*, (
    SELECT max(d.started_at)
    FROM deployments d
    JOIN applications a ON a.id = d.application_id
    WHERE a.project_id = p.id
) AS last_deployed_at
FROM projects p
ORDER BY p.created_at DESC;

-- name: UpdateProjectOwner :one
UPDATE projects SET user_id = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateProjectSharing :one
-- Owner/admin only — sharing is a destructive-adjacent right, not something a
-- shared member gains just by having access.
UPDATE projects SET shared = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;
