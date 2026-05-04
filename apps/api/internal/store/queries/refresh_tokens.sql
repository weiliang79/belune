-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (user_id, token_hash, expires_at, user_agent, ip)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetActiveRefreshToken :one
SELECT * FROM refresh_tokens
WHERE token_hash = $1
  AND revoked_at IS NULL
  AND expires_at > NOW();

-- name: TouchRefreshToken :exec
UPDATE refresh_tokens SET last_used_at = NOW() WHERE id = $1;

-- name: RevokeRefreshTokenByHash :exec
UPDATE refresh_tokens SET revoked_at = NOW()
WHERE token_hash = $1 AND revoked_at IS NULL;

-- name: RevokeRefreshTokenByID :exec
UPDATE refresh_tokens SET revoked_at = NOW()
WHERE id = $1 AND revoked_at IS NULL;

-- name: RevokeAllUserRefreshTokens :exec
UPDATE refresh_tokens SET revoked_at = NOW()
WHERE user_id = $1 AND revoked_at IS NULL;

-- name: DeleteExpiredRefreshTokens :exec
DELETE FROM refresh_tokens
WHERE expires_at < NOW() - INTERVAL '7 days';
