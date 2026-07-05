-- Database restores: record a single restore run so the UI can show restore
-- history alongside backups. Mirrors application_volume_restores (000031).
-- Previously DB restore was fire-and-forget (owner notification only); this
-- gives it its own audit/log trail.
CREATE TABLE database_restores (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    database_id  UUID NOT NULL REFERENCES databases(id) ON DELETE CASCADE,
    backup_id    UUID REFERENCES database_backups(id) ON DELETE SET NULL,
    started_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at  TIMESTAMPTZ,
    status       TEXT NOT NULL DEFAULT 'running'
        CHECK (status IN ('running', 'succeeded', 'failed')),
    error        TEXT,
    log          TEXT
);

CREATE INDEX idx_database_restores_database ON database_restores(database_id);
CREATE INDEX idx_database_restores_backup ON database_restores(backup_id);
