-- name: ListDomainsByService :many
SELECT * FROM domains WHERE service_id = $1 ORDER BY created_at DESC;

-- name: CreateDomain :one
INSERT INTO domains (service_id, hostname, ssl_enabled)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetDomain :one
SELECT * FROM domains WHERE id = $1;

-- name: DeleteDomain :exec
DELETE FROM domains WHERE id = $1;
