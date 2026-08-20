-- Opt-in TOTP second factor.
--
-- The secret is written at enrollment and the factor is only *enabled* once the
-- user proves they can generate a code from it, so these are two columns rather
-- than one: a secret with no totp_enabled_at is a scan in progress, and it must
-- not gate anything. Enabling on generation instead would lock out anyone whose
-- scan silently failed, immediately and permanently.
ALTER TABLE users
    ADD COLUMN totp_secret_encrypted BYTEA,
    -- NULL = not enabled. A timestamp rather than a boolean because when the
    -- factor was turned on is worth knowing, and NULL here is a fact, not a
    -- sentinel standing in for one.
    ADD COLUMN totp_enabled_at       TIMESTAMPTZ,
    -- Replay guard: the last time-step accepted for this user. A code stays
    -- valid for its whole window, so without this a code observed once (over a
    -- shoulder, in a log, on a phishing page) can be replayed for ~30 seconds.
    ADD COLUMN totp_last_step        BIGINT;

-- Recovery codes as rows, not an array on the user: used_at makes single-use
-- natural to enforce and to show ("6 of 10 remaining"), and it keeps a used
-- code visible instead of deleting the evidence that one was spent.
CREATE TABLE user_recovery_codes (
    id      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    -- SHA-256, not bcrypt: these carry ~80 bits of entropy from a CSPRNG, so
    -- there is no dictionary to slow down and nothing for a work factor to buy.
    code_hash  BYTEA NOT NULL,
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_user_recovery_codes_user ON user_recovery_codes(user_id);
