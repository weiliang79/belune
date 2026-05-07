-- Phase 4 of v0.0.10-alpha: per-user alert preferences.
--   Default-on (opt-out) for alpha: all alert types start enabled.
--   quota_threshold_percent controls the rising-edge trigger level.

CREATE TABLE alert_preferences (
    user_id                  UUID    PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    deploy_failures          BOOLEAN NOT NULL DEFAULT TRUE,
    build_failures           BOOLEAN NOT NULL DEFAULT TRUE,
    quota_threshold          BOOLEAN NOT NULL DEFAULT TRUE,
    quota_threshold_percent  INTEGER NOT NULL DEFAULT 80
        CHECK (quota_threshold_percent BETWEEN 1 AND 100)
);
