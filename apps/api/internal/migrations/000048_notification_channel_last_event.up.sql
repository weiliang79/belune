-- Record which event was last delivered (or last failed) on a channel, so the
-- UI can show "Sent <event> · <time>" rather than a bare timestamp. Stamped
-- alongside last_sent_at / last_error at delivery time; NULL until first use.
ALTER TABLE notification_channels
    ADD COLUMN last_event_type TEXT;
