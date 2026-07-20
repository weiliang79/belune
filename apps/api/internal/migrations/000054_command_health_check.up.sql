-- Command-based health checks, in addition to the existing HTTP probe.
--
-- The HTTP probe (health_check_path) is a one-shot readiness gate the control
-- plane runs after a deploy: it reaches the container over the Docker network
-- and only works for HTTP services. A command health check is a native Docker
-- HEALTHCHECK — a command run *inside* the container, continuously — so it works
-- for anything (databases, queues, workers) and keeps reporting after the
-- deploy, which lets the container's health drive the application's status.
--
-- health_check_type selects the mechanism. It defaults to 'http', which is what
-- every existing application already does (an empty path under 'http' is simply
-- skipped, exactly as before), so this migration changes no existing behaviour.

ALTER TABLE applications
    ADD COLUMN health_check_type TEXT NOT NULL DEFAULT 'http'
        CHECK (health_check_type IN ('none', 'http', 'command')),
    -- The command is stored as a shell string and run via CMD-SHELL, which is
    -- what a user expects when they type `curl -f localhost:3000/health`.
    ADD COLUMN health_check_command TEXT,
    -- Docker HEALTHCHECK knobs. NULL means "use the platform default" and is
    -- resolved when the container is created, not here, so the defaults live in
    -- one place. health_check_timeout_seconds (added in 000046) is reused as the
    -- per-check timeout for both mechanisms.
    ADD COLUMN health_check_interval_seconds     INTEGER,
    ADD COLUMN health_check_retries              INTEGER,
    ADD COLUMN health_check_start_period_seconds INTEGER;

-- 'unhealthy' is a running container that is failing its check — distinct from
-- 'error' (a crash or a failed deploy, i.e. not serving) and from 'stopped' (a
-- deliberate stop). Conflating it with 'error' would make an app that is up and
-- serving degraded traffic look identical to one that is down.
ALTER TABLE applications DROP CONSTRAINT IF EXISTS applications_status_check;
ALTER TABLE applications
    ADD CONSTRAINT applications_status_check
    CHECK (status IN ('inactive', 'running', 'stopped', 'error', 'unhealthy'));
