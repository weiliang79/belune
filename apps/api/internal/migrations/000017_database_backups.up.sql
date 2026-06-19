-- Per-database backup runs. Each row tracks one logical-dump backup of a managed
-- database (pg_dump / mysqldump / mongodump). The dump is gzipped to a local
-- file and, when remote backup is enabled, uploaded to S3 (remote_key).
CREATE TABLE database_backups (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    database_id UUID NOT NULL REFERENCES databases(id) ON DELETE CASCADE,
    started_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ,
    status      TEXT NOT NULL DEFAULT 'running'
        CHECK (status IN ('running', 'succeeded', 'failed')),
    local_path  TEXT,
    remote_key  TEXT,
    size_bytes  BIGINT NOT NULL DEFAULT 0,
    error       TEXT
);

CREATE INDEX idx_database_backups_database_id ON database_backups(database_id, started_at DESC);
