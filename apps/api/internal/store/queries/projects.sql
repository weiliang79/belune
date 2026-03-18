-- name: ListProjectsByUser :many
SELECT * FROM projects WHERE user_id = $1 ORDER BY created_at DESC;

-- name: GetProject :one
SELECT * FROM projects WHERE id = $1;

-- name: CreateProject :one
INSERT INTO projects (name, slug, user_id)
VALUES ($1, $2, $3)
RETURNING *;

-- name: UpdateProject :one
UPDATE projects SET name = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteProject :exec
DELETE FROM projects WHERE id = $1;

-- name: CountProjects :one
SELECT count(*) FROM projects;
