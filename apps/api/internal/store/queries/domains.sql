-- name: ListDomainsByApplication :many
SELECT * FROM domains WHERE application_id = $1 ORDER BY created_at DESC;

-- name: CreateDomain :one
INSERT INTO domains (application_id, hostname, ssl_enabled)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetDomain :one
SELECT * FROM domains WHERE id = $1;

-- name: DeleteDomain :exec
DELETE FROM domains WHERE id = $1;

-- name: GetDomainOwnerUserID :one
SELECT p.user_id FROM domains d
JOIN applications a ON a.id = d.application_id
JOIN projects p ON p.id = a.project_id
WHERE d.id = $1;
