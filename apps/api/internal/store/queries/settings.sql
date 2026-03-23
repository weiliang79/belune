-- name: GetSetting :one
SELECT * FROM settings WHERE key = $1;

-- name: UpsertSetting :one
INSERT INTO settings (key, value)
VALUES ($1, $2)
ON CONFLICT (key) DO UPDATE SET value = $2, updated_at = NOW()
RETURNING *;

-- name: ListSettings :many
SELECT * FROM settings ORDER BY key;
