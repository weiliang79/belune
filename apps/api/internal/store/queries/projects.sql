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

-- name: ListProjectsByUserLimit :many
-- Bounded sibling of ListProjectsByUser — see ListAllProjectsLimit for why
-- the pin is applied in SQL here rather than post-query in Go.
SELECT p.*, (
    SELECT max(d.started_at)
    FROM deployments d
    JOIN applications a ON a.id = d.application_id
    WHERE a.project_id = p.id
) AS last_deployed_at
FROM projects p
WHERE (p.user_id = sqlc.arg('user_id') OR p.shared)
  AND (sqlc.narg('project_id')::uuid IS NULL OR p.id = sqlc.narg('project_id'))
ORDER BY p.created_at DESC
LIMIT sqlc.arg('row_limit');

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

-- name: ListAllProjectsLimit :many
-- Bounded sibling of ListAllProjects for the MCP list_projects tool.
-- project_id is the PAT pin, applied here in SQL — not as a post-query
-- Go-side filter the way REST's ListProjects narrows it — so LIMIT can
-- never truncate away the one project a pinned token is allowed to see
-- before the pin gets a chance to narrow the result to it. Additive: REST
-- keeps using the unbounded query above unchanged.
SELECT p.*, (
    SELECT max(d.started_at)
    FROM deployments d
    JOIN applications a ON a.id = d.application_id
    WHERE a.project_id = p.id
) AS last_deployed_at
FROM projects p
WHERE (sqlc.narg('project_id')::uuid IS NULL OR p.id = sqlc.narg('project_id'))
ORDER BY p.created_at DESC
LIMIT sqlc.arg('row_limit');

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
