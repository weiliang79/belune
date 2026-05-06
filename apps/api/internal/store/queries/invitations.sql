-- name: CreateInvitation :one
INSERT INTO invitations (email, role, token_hash, invited_by_user_id, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetInvitationByHash :one
SELECT * FROM invitations WHERE token_hash = $1;

-- name: GetInvitationByEmailPending :one
SELECT * FROM invitations
WHERE email = $1 AND accepted_at IS NULL
LIMIT 1;

-- name: MarkInvitationAccepted :exec
UPDATE invitations
SET accepted_at = NOW(), accepted_user_id = $2
WHERE id = $1;

-- name: ListPendingInvitations :many
SELECT * FROM invitations
WHERE accepted_at IS NULL AND expires_at > NOW()
ORDER BY created_at DESC;

-- name: RevokeInvitation :exec
DELETE FROM invitations WHERE id = $1;

-- name: DeleteExpiredInvitations :exec
DELETE FROM invitations
WHERE expires_at < NOW() - INTERVAL '24 hours';
