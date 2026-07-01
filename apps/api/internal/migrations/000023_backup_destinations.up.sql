-- Project-scoped backup destinations + per-database backup configurations.
--
-- backup_destinations are S3-compatible storage targets owned by a project and
-- managed by its members (self-service, per-project credential isolation). They
-- are distinct from the global control-plane backup target (BACKUP_S3_* env
-- vars used by backup.sh) — different trust boundary, never shared.
--
-- database_backup_configs attach a schedule + retention to a managed database,
-- pointing at one destination. A database may have several configs (e.g. daily
-- to one bucket, weekly to another).

CREATE TABLE backup_destinations (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id            UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name                  TEXT NOT NULL,
    -- Informational provider label (drives the icon/help text in the UI); the
    -- transport is always S3-compatible regardless of value.
    provider              TEXT NOT NULL DEFAULT 's3'
        CHECK (provider IN ('s3', 'r2', 'b2', 'wasabi', 'minio', 'other')),
    endpoint              TEXT NOT NULL DEFAULT '',          -- empty = AWS regional endpoint
    region                TEXT NOT NULL DEFAULT 'us-east-1',
    bucket                TEXT NOT NULL,
    use_ssl               BOOLEAN NOT NULL DEFAULT true,
    -- keyring-encrypted JSON {access_key, secret_key}
    credentials_encrypted BYTEA NOT NULL,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_backup_destinations_project_id ON backup_destinations(project_id);

CREATE TABLE database_backup_configs (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    database_id    UUID NOT NULL REFERENCES databases(id) ON DELETE CASCADE,
    destination_id UUID NOT NULL REFERENCES backup_destinations(id) ON DELETE RESTRICT,
    prefix         TEXT NOT NULL DEFAULT '',  -- key prefix inside the bucket
    schedule       TEXT NOT NULL,             -- cron expression (robfig/cron)
    keep_latest    INT,                       -- NULL = keep all
    enabled        BOOLEAN NOT NULL DEFAULT true,
    last_run_at    TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_database_backup_configs_database_id ON database_backup_configs(database_id);
CREATE INDEX idx_database_backup_configs_enabled ON database_backup_configs(enabled) WHERE enabled;

-- Link a recorded run to the config that produced it (NULL = manual ad-hoc run).
ALTER TABLE database_backups
    ADD COLUMN backup_config_id UUID REFERENCES database_backup_configs(id) ON DELETE SET NULL;

CREATE INDEX idx_database_backups_backup_config_id ON database_backups(backup_config_id);
