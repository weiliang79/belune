-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: CountUsers :one
SELECT COUNT(*) FROM users;

-- name: CreateUser :one
INSERT INTO users (email, password_hash, role)
VALUES ($1, $2, $3)
RETURNING *;

-- name: ListUsers :many
SELECT id, email, role, created_at FROM users ORDER BY created_at ASC;

-- name: UpdateUserRole :one
UPDATE users SET role = $2 WHERE id = $1
RETURNING id, email, role, created_at;

-- name: UpdateUserPassword :exec
UPDATE users SET password_hash = $2 WHERE id = $1;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = $1;

-- name: CountAdmins :one
SELECT COUNT(*) FROM users WHERE role = 'admin';
