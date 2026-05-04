-- Phase 1 of v0.0.9-alpha: auth lifecycle.
--   * refresh_tokens: DB-backed session records. Survives Redis restart.
--     token_hash is SHA-256 of the opaque plaintext returned to the client;
--     plaintext is never persisted.
--   * login_attempts: per-email failed-login counters used to enforce
--     account lockout after repeated bad-credential attempts.

CREATE TABLE refresh_tokens (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash   BYTEA NOT NULL UNIQUE,
    expires_at   TIMESTAMPTZ NOT NULL,
    revoked_at   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ,
    user_agent   TEXT NOT NULL DEFAULT '',
    ip           TEXT NOT NULL DEFAULT ''
);

CREATE INDEX idx_refresh_tokens_user_active
    ON refresh_tokens(user_id)
    WHERE revoked_at IS NULL;

CREATE INDEX idx_refresh_tokens_expires_at
    ON refresh_tokens(expires_at)
    WHERE revoked_at IS NULL;

CREATE TABLE login_attempts (
    email           VARCHAR(255) PRIMARY KEY,
    failed_count    INTEGER NOT NULL DEFAULT 0,
    first_failed_at TIMESTAMPTZ,
    locked_until    TIMESTAMPTZ
);
