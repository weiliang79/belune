-- Git provider integrations: replaces the centralized git_credentials table
-- (PAT-only, no repo/account association) with a two-tier model:
--   * generic/PAT auth lives inline on the application (git_credentials_encrypted)
--   * reusable provider account connections live in git_integrations, configured
--     against a per-instance provider app/oauth client in git_provider_configs.

-- Per-instance registered provider app (GitHub App) or OAuth client. Admin-owned.
-- Keyed by (provider, base_url) so multiple self-hosted instances are allowed.
CREATE TABLE git_provider_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider VARCHAR(50) NOT NULL CHECK (provider IN ('github','gitlab','bitbucket','gitea')),
    base_url VARCHAR(255) NOT NULL DEFAULT '',
    client_id VARCHAR(255) NOT NULL DEFAULT '',
    app_id VARCHAR(255) NOT NULL DEFAULT '',
    app_slug VARCHAR(255) NOT NULL DEFAULT '',
    -- JSON blob, keyring-encrypted: OAuth client secret, or GitHub App private key
    -- + webhook secret.
    secret_encrypted BYTEA,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (provider, base_url)
);

-- Per-user account connection. One row = one connected GitHub App installation or
-- OAuth account. config_encrypted holds the means to mint a short-lived clone token
-- (JSON: github -> installation_id; oauth -> access+refresh token + expiry).
CREATE TABLE git_integrations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider VARCHAR(50) NOT NULL CHECK (provider IN ('github','gitlab','bitbucket','gitea')),
    base_url VARCHAR(255) NOT NULL DEFAULT '',
    account_login VARCHAR(255) NOT NULL DEFAULT '',
    config_encrypted BYTEA NOT NULL,
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_git_integrations_created_by ON git_integrations(created_by);

-- Applications reference a connection by FK; generic PAT stays inline.
ALTER TABLE applications
    ADD COLUMN git_integration_id UUID REFERENCES git_integrations(id) ON DELETE SET NULL;

-- Migrate existing centralized PATs down to the app level. The encrypted bytes are
-- keyring-compatible as-is, so this is a plain byte copy (no decrypt/re-encrypt).
UPDATE applications a
SET git_credentials_encrypted = c.token_encrypted
FROM git_credentials c
WHERE a.git_credential_id = c.id
  AND (a.git_credentials_encrypted IS NULL OR length(a.git_credentials_encrypted) = 0);

ALTER TABLE applications DROP COLUMN git_credential_id;

DROP TABLE git_credentials;
