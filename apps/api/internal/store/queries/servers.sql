-- name: GetLocalServer :one
SELECT * FROM servers WHERE is_local LIMIT 1;

-- name: ListManagedServers :many
-- Every server Belune still manages, for sweeps that must cover each host.
-- Revoked servers are deliberately excluded: Forget is a revocation rather than
-- a deletion, so that host's containers are no longer ours to act on.
SELECT * FROM servers WHERE lifecycle <> 'revoked' ORDER BY created_at;
