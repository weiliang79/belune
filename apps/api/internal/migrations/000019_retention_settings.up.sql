-- Seed retention settings for the Server-page retention panel.
-- audit_log_retention_days defaults to '0' = keep forever (audit logs are
-- compliance-sensitive and not pruned unless an operator opts in).
-- deploy_history_retention_days defaults to '90' days.
INSERT INTO settings (key, value) VALUES
    ('audit_log_retention_days', '0'),
    ('deploy_history_retention_days', '90')
ON CONFLICT (key) DO NOTHING;
