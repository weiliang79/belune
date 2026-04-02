-- 3A: User profile fields
ALTER TABLE users ADD COLUMN username VARCHAR(100) NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN first_name VARCHAR(100) NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN last_name VARCHAR(100) NOT NULL DEFAULT '';

-- 3B: HTTP request logs (populated by Caddy access log tailer)
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

-- 3C: Application container logs (populated by Docker log collector)
CREATE TABLE application_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    stream VARCHAR(10) NOT NULL DEFAULT 'stdout',
    message TEXT NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_app_logs_app_recorded ON application_logs(application_id, recorded_at DESC);

-- 3D: Store image tag on deployments for rollback support
ALTER TABLE deployments ADD COLUMN image_tag VARCHAR(500);

-- Expand triggered_by CHECK to support rollback (needed in Phase 6)
ALTER TABLE deployments DROP CONSTRAINT IF EXISTS deployments_triggered_by_check;
ALTER TABLE deployments ADD CONSTRAINT deployments_triggered_by_check
    CHECK (triggered_by IN ('push', 'manual', 'api', 'rollback'));
