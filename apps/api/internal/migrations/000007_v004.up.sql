-- v0.0.4-alpha schema changes

-- ============================================================
-- 1. Centralized Git Credentials (multi-provider)
-- ============================================================
CREATE TABLE git_credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    provider VARCHAR(50) NOT NULL CHECK (provider IN ('github','gitlab','bitbucket','generic')),
    token_encrypted BYTEA NOT NULL,
    username VARCHAR(255) NOT NULL DEFAULT '',
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_git_credentials_created_by ON git_credentials(created_by);

-- Link applications to centralized credentials (nullable, backward-compatible)
ALTER TABLE applications ADD COLUMN IF NOT EXISTS git_credential_id UUID REFERENCES git_credentials(id) ON DELETE SET NULL;

-- ============================================================
-- 2. Domain Configuration Expansion
-- ============================================================
ALTER TABLE domains ADD COLUMN IF NOT EXISTS container_port INTEGER;
ALTER TABLE domains ADD COLUMN IF NOT EXISTS force_https BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE domains ADD COLUMN IF NOT EXISTS ssl_mode VARCHAR(50) NOT NULL DEFAULT 'automatic'
    CHECK (ssl_mode IN ('automatic','dns_challenge','custom','off'));
ALTER TABLE domains ADD COLUMN IF NOT EXISTS ssl_provider VARCHAR(100);
ALTER TABLE domains ADD COLUMN IF NOT EXISTS ssl_credentials_encrypted BYTEA;
ALTER TABLE domains ADD COLUMN IF NOT EXISTS cert_path VARCHAR(500);
ALTER TABLE domains ADD COLUMN IF NOT EXISTS key_path VARCHAR(500);
ALTER TABLE domains ADD COLUMN IF NOT EXISTS advanced_config JSONB;

-- Route features per domain (basic_auth, redirects, headers, IP allowlist, rate_limit)
CREATE TABLE domain_route_features (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id UUID NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    feature_type VARCHAR(50) NOT NULL
        CHECK (feature_type IN ('basic_auth','redirect','headers','ip_allowlist','rate_limit')),
    config JSONB NOT NULL DEFAULT '{}',
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(domain_id, feature_type)
);

CREATE INDEX idx_route_features_domain ON domain_route_features(domain_id);

-- ============================================================
-- 3. Audit Logs
-- ============================================================
CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(50) NOT NULL,
    resource_id VARCHAR(255),
    details JSONB,
    ip_address VARCHAR(45),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at DESC);
CREATE INDEX idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX idx_audit_logs_resource ON audit_logs(resource_type, resource_id);
