-- name: GetLocalServer :one
SELECT * FROM servers WHERE is_local LIMIT 1;
