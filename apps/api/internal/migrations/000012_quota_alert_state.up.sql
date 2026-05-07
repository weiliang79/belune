-- Phase 4 of v0.0.10-alpha: per-scope quota alert state.
--   Tracks the last usage-percent at which an alert fired so the rising-edge
--   + hysteresis logic can suppress duplicate notifications.

CREATE TABLE quota_alert_state (
    scope                TEXT        NOT NULL,
    scope_id             UUID        NOT NULL,
    last_alerted_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_alerted_percent INTEGER     NOT NULL DEFAULT 0,
    PRIMARY KEY (scope, scope_id)
);
