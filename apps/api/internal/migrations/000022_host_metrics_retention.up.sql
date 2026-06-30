-- Host metrics are stored at 1-second granularity, so they use a short,
-- hours-based retention window (default 24h) pruned hourly, instead of the
-- day-based metrics_retention_days (now superseded).
--
-- request_log_retention_days seeds the Request-logs retention row at 3 days.
INSERT INTO settings (key, value) VALUES
    ('host_metrics_retention_hours', '24'),
    ('request_log_retention_days', '3')
ON CONFLICT (key) DO NOTHING;
