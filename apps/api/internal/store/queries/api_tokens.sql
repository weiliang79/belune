-- name: CreateAPIToken :one
INSERT INTO api_tokens (user_id, name, token_hash, scopes, project_id, role_at_issue, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetAPITokenByHash :one
-- The auth-path lookup: the token row plus the owner's CURRENT role, so the
-- effective role (min(role_at_issue, user_role)) can be computed without a
-- second query. A revoked/deleted user cascades their tokens away (ON DELETE
-- CASCADE), so a row returned here always has a live owner.
SELECT t.*, u.role AS user_role
FROM api_tokens t
JOIN users u ON u.id = t.user_id
WHERE t.token_hash = $1;

-- name: UpdateAPITokenLastUsed :exec
UPDATE api_tokens SET last_used_at = $2 WHERE id = $1;
