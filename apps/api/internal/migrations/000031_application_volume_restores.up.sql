-- Application volume restores: record a single restore run so the UI can show
-- progress/outcome. The backup path already records runs
-- (application_volume_backups); restores previously had no row (completion was
-- the only signal, matching the managed-database restore). This table gives
-- restore its own audit/log trail without touching the DB restore path.
CREATE TABLE application_volume_restores (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_volume_id UUID NOT NULL REFERENCES application_volumes(id) ON DELETE CASCADE,
    backup_id             UUID REFERENCES application_volume_backups(id) ON DELETE SET NULL,
    started_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at           TIMESTAMPTZ,
    status                TEXT NOT NULL DEFAULT 'running'
        CHECK (status IN ('running', 'succeeded', 'failed')),
    error                 TEXT,
    log                   TEXT
);

CREATE INDEX idx_avr_volume ON application_volume_restores(application_volume_id);
CREATE INDEX idx_avr_backup ON application_volume_restores(backup_id);
