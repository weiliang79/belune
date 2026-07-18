-- Per-application health-check tuning, populated by templates whose manifest
-- declares a structured health_check block. Both NULL keep the platform
-- defaults: retry for healthVerifyTimeout (120s) and accept any 2xx.
ALTER TABLE applications
    ADD COLUMN health_check_timeout_seconds INTEGER,
    ADD COLUMN health_check_expect_status   INTEGER;
