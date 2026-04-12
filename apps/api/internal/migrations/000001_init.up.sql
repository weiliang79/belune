-- ============================================================
-- Consolidated init migration (v0.0.5-alpha)
-- Merges: 000001 through 000008
-- ============================================================

-- Users
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL DEFAULT 'admin',
    username VARCHAR(100) NOT NULL DEFAULT '',
    first_name VARCHAR(100) NOT NULL DEFAULT '',
    last_name VARCHAR(100) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Projects
CREATE TABLE projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) NOT NULL UNIQUE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_projects_user_id ON projects(user_id);

-- Centralized Git Credentials (multi-provider)
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

-- Applications
CREATE TABLE applications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL CHECK (type IN ('git', 'image')),
    source_repo TEXT,
    source_image TEXT,
    dockerfile_path VARCHAR(500) DEFAULT 'Dockerfile',
    build_type VARCHAR(50) NOT NULL CHECK (build_type IN ('dockerfile', 'buildpacks', 'railpack', 'image')),
    build_type_override VARCHAR(50) CHECK (build_type_override IN ('dockerfile', 'buildpacks', 'railpack', 'image')),
    builder_image VARCHAR(500),
    custom_buildpacks JSONB DEFAULT '[]',
    cpu_limit DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (cpu_limit >= 0),
    memory_limit BIGINT NOT NULL DEFAULT 0,
    webhook_secret VARCHAR(255),
    auto_deploy_branch VARCHAR(255) DEFAULT 'main',
    status VARCHAR(50) NOT NULL DEFAULT 'inactive',
    git_credentials_encrypted BYTEA,
    health_check_path VARCHAR(255),
    git_credential_id UUID REFERENCES git_credentials(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_applications_project_id ON applications(project_id);
CREATE UNIQUE INDEX idx_applications_slug_per_project ON applications(project_id, slug);
CREATE INDEX idx_applications_source_repo ON applications(source_repo);

-- Deployments
CREATE TABLE deployments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    status VARCHAR(50) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'building', 'deploying', 'success', 'failed')),
    triggered_by VARCHAR(50) NOT NULL CHECK (triggered_by IN ('push', 'manual', 'api', 'rollback')),
    commit_sha VARCHAR(40),
    image_tag VARCHAR(500),
    build_logs TEXT,
    error_message TEXT,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ
);

CREATE INDEX idx_deployments_app_started ON deployments(application_id, started_at DESC);

-- Databases
CREATE TABLE databases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL CHECK (type IN ('postgres', 'mysql', 'redis', 'mongo')),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) NOT NULL,
    version VARCHAR(50) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'creating',
    internal_host VARCHAR(255),
    internal_port INTEGER,
    credentials_encrypted BYTEA,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_databases_project_id ON databases(project_id);
CREATE UNIQUE INDEX idx_databases_slug_per_project ON databases(project_id, slug);

-- Domains
CREATE TABLE domains (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    hostname VARCHAR(255) NOT NULL UNIQUE,
    ssl_enabled BOOLEAN NOT NULL DEFAULT true,
    caddy_config_id VARCHAR(255),
    container_port INTEGER,
    force_https BOOLEAN NOT NULL DEFAULT true,
    ssl_mode VARCHAR(50) NOT NULL DEFAULT 'automatic'
        CHECK (ssl_mode IN ('automatic','dns_challenge','custom','off')),
    ssl_provider VARCHAR(100),
    ssl_credentials_encrypted BYTEA,
    cert_path VARCHAR(500),
    key_path VARCHAR(500),
    advanced_config JSONB,
    verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_domains_application_id ON domains(application_id);
CREATE UNIQUE INDEX idx_domains_hostname ON domains(hostname);

-- Domain route features (basic_auth, redirects, headers, IP allowlist, rate_limit)
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

-- Environment Variables (Application-level)
CREATE TABLE env_vars (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    key VARCHAR(255) NOT NULL,
    value_encrypted BYTEA NOT NULL,
    is_secret BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(application_id, key)
);

CREATE INDEX idx_env_vars_application_id ON env_vars(application_id);

-- Environment Variables (Project-level)
CREATE TABLE project_env_vars (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    key VARCHAR(255) NOT NULL,
    value_encrypted BYTEA NOT NULL,
    is_secret BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(project_id, key)
);

CREATE INDEX idx_project_env_vars_project_id ON project_env_vars(project_id);

-- HTTP request logs (populated by Caddy access log tailer)
CREATE TABLE request_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    method VARCHAR(10) NOT NULL,
    path TEXT NOT NULL,
    status_code SMALLINT NOT NULL,
    latency_ms INTEGER NOT NULL,
    hostname VARCHAR(255) NOT NULL,
    request_size INTEGER,
    response_size INTEGER,
    client_ip VARCHAR(45),
    user_agent TEXT,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_request_logs_app_recorded ON request_logs(application_id, recorded_at DESC);
CREATE INDEX idx_request_logs_recorded ON request_logs(recorded_at DESC);

-- Application container logs (populated by Docker log collector)
CREATE TABLE application_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    stream VARCHAR(10) NOT NULL DEFAULT 'stdout',
    message TEXT NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_app_logs_app_recorded ON application_logs(application_id, recorded_at DESC);

-- Host Metrics (stored every 60s)
CREATE TABLE host_metrics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cpu_percent DOUBLE PRECISION NOT NULL,
    memory_used BIGINT NOT NULL,
    memory_total BIGINT NOT NULL,
    disk_used BIGINT NOT NULL,
    disk_total BIGINT NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_host_metrics_recorded_at ON host_metrics(recorded_at DESC);

-- Audit Logs
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

-- System Settings
CREATE TABLE settings (
    key VARCHAR(255) PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO settings (key, value) VALUES ('metrics_retention_days', '14');
