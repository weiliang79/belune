CREATE TABLE backup_runs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    started_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ,
    status      TEXT NOT NULL DEFAULT 'running',
    local_path  TEXT NOT NULL DEFAULT '',
    remote_key  TEXT,
    size_bytes  BIGINT NOT NULL DEFAULT 0,
    error       TEXT
);
