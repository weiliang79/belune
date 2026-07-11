-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = $1;

-- name: CountUsers :one
SELECT COUNT(*) FROM users;

-- name: CreateUser :one
INSERT INTO users (email, password_hash, role, username, first_name, last_name)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListUsers :many
SELECT u.id, u.email, u.role, u.username, u.first_name, u.last_name, u.created_at,
       (SELECT max(rt.last_used_at) FROM refresh_tokens rt WHERE rt.user_id = u.id) AS last_active_at
FROM users u
ORDER BY u.created_at ASC;

-- name: UpdateUserRole :one
UPDATE users SET role = $2 WHERE id = $1
RETURNING id, email, role, username, first_name, last_name, created_at;

-- name: UpdateUserProfile :one
UPDATE users SET username = $2, first_name = $3, last_name = $4
WHERE id = $1
RETURNING id, email, role, username, first_name, last_name, created_at;

-- name: UpdateUserPassword :exec
UPDATE users SET password_hash = $2 WHERE id = $1;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = $1;

-- name: CountAdmins :one
SELECT COUNT(*) FROM users WHERE role = 'admin';

-- name: ListAdminUserIDs :many
-- Recipients for instance-wide alerts (TLS failures, expiries) that belong to no
-- single project owner.
SELECT id FROM users WHERE role = 'admin';
