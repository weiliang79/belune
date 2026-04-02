-- Replace single-column index with composite for sort optimization
DROP INDEX IF EXISTS idx_deployments_application_id;
CREATE INDEX idx_deployments_app_started ON deployments(application_id, started_at DESC);

-- Webhook lookup by repo
CREATE INDEX IF NOT EXISTS idx_applications_source_repo ON applications(source_repo);

-- Host metrics time-range queries (DESC for recent-first)
DROP INDEX IF EXISTS idx_host_metrics_recorded_at;
CREATE INDEX idx_host_metrics_recorded_at ON host_metrics(recorded_at DESC);
