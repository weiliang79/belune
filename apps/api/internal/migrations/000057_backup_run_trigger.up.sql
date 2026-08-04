-- Records who started a control-plane backup run: the in-app worker (manual
-- click or the cron sweep) or the host CLI (scripts/backup.sh, run directly or
-- via `systemctl start belune-backup.service`). Both producers now write the
-- same archive format and both record a run here, so this column is what lets
-- "Recent Runs" attribute each row instead of implying they all came from one
-- source.
ALTER TABLE backup_runs
    ADD COLUMN trigger TEXT NOT NULL DEFAULT 'worker';
