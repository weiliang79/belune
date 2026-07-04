-- Application volume backups: snapshot an app's persistent volume to an
-- S3-compatible destination and restore it. Generalises the managed-database
-- backup subsystem (backup_destinations, database_backup_configs,
-- database_backups) to application_volumes.
--
-- A config attaches a destination (+ options) to one application volume; a
-- backup row records a single snapshot run. quiesce controls whether the app
-- container is stopped for a consistent snapshot (default off = hot snapshot,
-- no downtime). schedule/keep_latest exist for the Phase-2 scheduler + prune
-- but are unused by the MVP (manual "Back up now" only).
CREATE TABLE application_volume_backup_configs (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_volume_id UUID NOT NULL REFERENCES application_volumes(id) ON DELETE CASCADE,
    destination_id        UUID NOT NULL REFERENCES backup_destinations(id) ON DELETE RESTRICT,
    prefix                TEXT NOT NULL DEFAULT '',    -- key prefix inside the bucket
    schedule              TEXT NOT NULL DEFAULT '',    -- cron expr (Phase 2; empty = manual only)
    keep_latest           INT,                         -- NULL = keep all (Phase 2 prune)
    enabled               BOOLEAN NOT NULL DEFAULT true,
    quiesce               BOOLEAN NOT NULL DEFAULT false, -- stop the app during the snapshot
    last_run_at           TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_avbc_volume ON application_volume_backup_configs(application_volume_id);
CREATE INDEX idx_avbc_destination ON application_volume_backup_configs(destination_id);

CREATE TABLE application_volume_backups (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_volume_id UUID NOT NULL REFERENCES application_volumes(id) ON DELETE CASCADE,
    backup_config_id      UUID REFERENCES application_volume_backup_configs(id) ON DELETE SET NULL,
    started_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at           TIMESTAMPTZ,
    status                TEXT NOT NULL DEFAULT 'running'
        CHECK (status IN ('running', 'succeeded', 'failed')),
    local_path            TEXT,
    remote_key            TEXT,
    size_bytes            BIGINT NOT NULL DEFAULT 0,
    error                 TEXT,
    log                   TEXT
);

CREATE INDEX idx_avb_volume ON application_volume_backups(application_volume_id);
CREATE INDEX idx_avb_config ON application_volume_backups(backup_config_id);
