-- name: ListDatabasesByProject :many
SELECT * FROM databases WHERE project_id = $1 ORDER BY created_at DESC;

-- name: GetDatabase :one
SELECT * FROM databases WHERE id = $1;

-- name: CreateDatabase :one
INSERT INTO databases (project_id, type, name, slug, version, status, internal_host, internal_port, credentials_encrypted)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: UpdateDatabaseStatus :one
UPDATE databases SET status = $2 WHERE id = $1 RETURNING *;

-- name: UpdateDatabaseAfterProvision :one
UPDATE databases SET status = $2, internal_host = $3, internal_port = $4 WHERE id = $1 RETURNING *;

-- name: DeleteDatabase :exec
DELETE FROM databases WHERE id = $1;

-- name: CountDatabases :one
SELECT count(*) FROM databases;
