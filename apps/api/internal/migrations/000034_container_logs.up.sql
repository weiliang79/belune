-- Unified container logs: replaces application_logs and adds database container
-- log capture. Each row is a single log line tagged with a severity level.
CREATE TABLE container_logs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_type VARCHAR(20) NOT NULL CHECK (source_type IN ('application', 'database')),
    source_id   UUID NOT NULL,
    level       VARCHAR(10) NOT NULL DEFAULT 'info',
    stream      VARCHAR(10) NOT NULL DEFAULT 'stdout',
    message     TEXT NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_container_logs_source
    ON container_logs (source_type, source_id, recorded_at DESC);

-- Carry existing application logs over (existing rows default to 'info'; their
-- level will be inferred going forward as new lines arrive).
INSERT INTO container_logs (source_type, source_id, level, stream, message, recorded_at)
SELECT 'application', application_id, 'info', stream, message, recorded_at
FROM application_logs;

DROP TABLE application_logs;
