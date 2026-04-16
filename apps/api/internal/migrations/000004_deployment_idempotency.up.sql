-- Deployment idempotency: opaque string derived from
-- application_id + trigger + commit_sha (or equivalent discriminator)
-- used to collapse duplicate webhook retries and rapid manual triggers
-- into a single deployment within a short time window.
ALTER TABLE deployments ADD COLUMN idempotency_key TEXT;

CREATE INDEX idx_deployments_app_idempotency_started
    ON deployments(application_id, idempotency_key, started_at DESC)
    WHERE idempotency_key IS NOT NULL;
