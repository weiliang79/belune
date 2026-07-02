-- Per-run log for the control-plane (system) backup: the full backup.sh output
-- (Postgres dump, Caddy certs, archive, upload) captured for every run so the
-- UI can show details in an expandable panel, on success as well as failure.
ALTER TABLE backup_runs
    ADD COLUMN log TEXT NOT NULL DEFAULT '';
