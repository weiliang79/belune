-- name: ListDatabasesByProject :many
SELECT * FROM databases WHERE project_id = $1 ORDER BY created_at DESC;

-- name: GetDatabase :one
SELECT * FROM databases WHERE id = $1;

-- name: CreateDatabase :one
INSERT INTO databases (project_id, type, name, version, status, internal_host, internal_port, credentials_encrypted)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: DeleteDatabase :exec
DELETE FROM databases WHERE id = $1;
