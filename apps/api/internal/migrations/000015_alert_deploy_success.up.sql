-- Dedicated preference for deploy *success* notifications, so they can be
-- silenced independently of deploy failures. Default-on (opt-out) for alpha,
-- matching the other alert toggles.
ALTER TABLE alert_preferences
    ADD COLUMN deploy_success BOOLEAN NOT NULL DEFAULT TRUE;
