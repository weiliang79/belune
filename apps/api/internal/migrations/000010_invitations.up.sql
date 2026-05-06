-- Phase 3 of v0.0.10-alpha: user invitations.
--   token_hash is SHA-256 of a 32-byte opaque plaintext sent in the invite
--   email; plaintext is never stored. Email is lowercased at write; the
--   partial unique index prevents duplicate pending invites for the same address.

CREATE TABLE invitations (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    email               TEXT        NOT NULL,
    role                TEXT        NOT NULL,
    token_hash          TEXT        NOT NULL UNIQUE,
    invited_by_user_id  UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at          TIMESTAMPTZ NOT NULL DEFAULT NOW() + INTERVAL '7 days',
    accepted_at         TIMESTAMPTZ,
    accepted_user_id    UUID        REFERENCES users(id) ON DELETE SET NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Prevents two pending invitations for the same address.
CREATE UNIQUE INDEX idx_invitations_email_pending
    ON invitations (email) WHERE accepted_at IS NULL;

CREATE INDEX idx_invitations_token_hash ON invitations (token_hash);
CREATE INDEX idx_invitations_expires_at ON invitations (expires_at);
