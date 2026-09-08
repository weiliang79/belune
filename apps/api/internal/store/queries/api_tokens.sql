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
-- Self-guarding: only writes when unset or older than the caller-supplied
-- coarsening threshold, so the write-coarsening window is one atomic
-- statement rather than a separate read-then-write race in Go (benign
-- either way, since the value only ever moves forward, but this removes the
-- redundant write entirely instead of relying on that).
UPDATE api_tokens
SET last_used_at = $2
WHERE id = $1
  AND (last_used_at IS NULL OR last_used_at < sqlc.arg('threshold')::timestamptz);

-- name: ListAPITokensByUser :many
-- The settings-page list: newest first, token_hash never selected — nothing
-- past the create response ever needs anything derived from it.
SELECT id, name, scopes, project_id, role_at_issue, expires_at, last_used_at, created_at
FROM api_tokens
WHERE user_id = $1
ORDER BY created_at DESC;

-- name: DeleteAPIToken :one
-- Scoped by user_id, not just id: pgx.ErrNoRows is how the handler tells
-- "not found" apart from "not yours" — both must read the same to the
-- caller, so there is no separate ownership lookup to get out of sync with
-- it. RETURNING name so the audit entry for the delete can carry it, the same
-- way create's does — one statement, not a second lookup.
DELETE FROM api_tokens WHERE id = $1 AND user_id = $2 RETURNING name;
