-- Per-run backup log: a short human-readable step log (dump output, sizes,
-- upload target, errors) captured for every run so the UI can show details.
ALTER TABLE database_backups
    ADD COLUMN log TEXT NOT NULL DEFAULT '';
