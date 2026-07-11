-- TLS status pipeline. Until now a certificate could silently fail to issue and
-- the UI would show nothing — the route goes live on :80 and the user is left
-- guessing why HTTPS never came up. These columns hold what the server actually
-- observes on the wire, so the badge reflects reality rather than inferring it
-- from the domain's configuration.
ALTER TABLE domains
    ADD COLUMN tls_status VARCHAR(20) NOT NULL DEFAULT 'unknown'
        CHECK (tls_status IN ('unknown', 'disabled', 'pending', 'active', 'expiring', 'expired', 'failed')),
    -- Read off the leaf certificate the proxy actually presents.
    ADD COLUMN tls_issuer TEXT,
    ADD COLUMN tls_not_after TIMESTAMPTZ,
    ADD COLUMN tls_last_checked_at TIMESTAMPTZ,
    -- Why issuance failed, in the user's words: an ACME error lifted out of
    -- Caddy's logs, or a DNS mismatch found before Caddy even tries.
    ADD COLUMN tls_error TEXT;

-- The probe worker sweeps by status to find domains needing attention.
CREATE INDEX idx_domains_tls_status ON domains (tls_status);

-- Caddy's own process logs now flow through the same collector as application
-- and database containers, which is what lets us lift ACME failure reasons out
-- of them. 000034 created this CHECK with only application|database.
ALTER TABLE container_logs
    DROP CONSTRAINT container_logs_source_type_check;

ALTER TABLE container_logs
    ADD CONSTRAINT container_logs_source_type_check
        CHECK (source_type IN ('application', 'database', 'system'));
