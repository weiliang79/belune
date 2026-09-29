-- name: CreateAPIToken :one
-- project_id is never written (see migration 000067) — a token's pins live
-- in api_token_projects, inserted separately by CreateAPITokenProjectPins in
-- the same transaction.
INSERT INTO api_tokens (user_id, name, token_hash, scopes, pinned, role_at_issue, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: CreateAPITokenProjectPins :exec
-- Inserts one row per id in project_ids for token_id. Called in the same
-- store.WithTx as CreateAPIToken, never on its own — a token row committed
-- without its pin rows would be pinned=true with zero pins, which is a
-- narrowed-to-nothing token, not the wide-open one a partial write for an
-- UNPINNED token would otherwise risk being mistaken for. Both directions of
-- that partial-write failure are wrong, which is why this must never run
-- outside the same transaction as the INSERT above.
INSERT INTO api_token_projects (token_id, project_id)
SELECT sqlc.arg('token_id')::uuid, unnest(sqlc.arg('project_ids')::uuid[]);

-- name: GetAPITokenByHash :one
-- The auth-path lookup: the token row, the owner's CURRENT role (so the
-- effective role can be computed without a second query), and the token's
-- full pin set via one LEFT JOIN + array_agg — this runs on EVERY request, so
-- the pin set is loaded here rather than with a second round trip. A revoked/
-- deleted user cascades their tokens away (ON DELETE CASCADE), so a row
-- returned here always has a live owner. project_ids is '{}' for an unpinned
-- token (pinned=false) exactly as often as for a pinned token with no
-- reachable projects left — the caller distinguishes those by pinned, never
-- by array length.
SELECT t.*, u.role AS user_role,
       COALESCE(array_agg(atp.project_id) FILTER (WHERE atp.project_id IS NOT NULL), '{}')::uuid[] AS project_ids
FROM api_tokens t
JOIN users u ON u.id = t.user_id
LEFT JOIN api_token_projects atp ON atp.token_id = t.id
WHERE t.token_hash = $1
GROUP BY t.id, u.role;

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
-- past the create response ever needs anything derived from it. project_ids
-- is every project this token is pinned to ('{}' when pinned is false).
SELECT t.id, t.name, t.scopes, t.pinned, t.role_at_issue, t.expires_at, t.last_used_at, t.created_at,
       COALESCE(array_agg(atp.project_id) FILTER (WHERE atp.project_id IS NOT NULL), '{}')::uuid[] AS project_ids
FROM api_tokens t
LEFT JOIN api_token_projects atp ON atp.token_id = t.id
WHERE t.user_id = $1
GROUP BY t.id
ORDER BY t.created_at DESC;

-- name: DeleteAPIToken :one
-- Scoped by user_id, not just id: pgx.ErrNoRows is how the handler tells
-- "not found" apart from "not yours" — both must read the same to the
-- caller, so there is no separate ownership lookup to get out of sync with
-- it. RETURNING name so the audit entry for the delete can carry it, the same
-- way create's does — one statement, not a second lookup. api_token_projects
-- rows for this token go with it via ON DELETE CASCADE.
DELETE FROM api_tokens WHERE id = $1 AND user_id = $2 RETURNING name;
