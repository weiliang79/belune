-- Users
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL DEFAULT 'admin',
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

-- Services
CREATE TABLE services (
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
    webhook_secret VARCHAR(255),
    auto_deploy_branch VARCHAR(255) DEFAULT 'main',
    status VARCHAR(50) NOT NULL DEFAULT 'inactive',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_services_project_id ON services(project_id);
CREATE UNIQUE INDEX idx_services_slug_per_project ON services(project_id, slug);

-- Deployments
CREATE TABLE deployments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id UUID NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    status VARCHAR(50) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'building', 'deploying', 'success', 'failed')),
    triggered_by VARCHAR(50) NOT NULL CHECK (triggered_by IN ('push', 'manual', 'api')),
    commit_sha VARCHAR(40),
    build_logs TEXT,
    error_message TEXT,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ
);

CREATE INDEX idx_deployments_service_id ON deployments(service_id);

-- Databases
CREATE TABLE databases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL CHECK (type IN ('postgres', 'mysql', 'redis', 'mongo')),
    name VARCHAR(255) NOT NULL,
    version VARCHAR(50) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'creating',
    internal_host VARCHAR(255),
    internal_port INTEGER,
    credentials_encrypted BYTEA,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_databases_project_id ON databases(project_id);

-- Domains
CREATE TABLE domains (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id UUID NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    hostname VARCHAR(255) NOT NULL UNIQUE,
    ssl_enabled BOOLEAN NOT NULL DEFAULT true,
    caddy_config_id VARCHAR(255),
    verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_domains_service_id ON domains(service_id);
CREATE UNIQUE INDEX idx_domains_hostname ON domains(hostname);

-- Environment Variables
CREATE TABLE env_vars (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id UUID NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    key VARCHAR(255) NOT NULL,
    value_encrypted BYTEA NOT NULL,
    is_secret BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(service_id, key)
);

CREATE INDEX idx_env_vars_service_id ON env_vars(service_id);
