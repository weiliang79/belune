-- Per-backup target database selector.
--   ''  -> the provisioned managed database (default, current behavior)
--   '*' -> all databases in the container (pg_dumpall / mysqldump --all-databases /
--          full mongodump) — a cluster-level backup
--   name -> a specific database in the same container (e.g. 'postgres')
-- The target is copied onto each run so restore replays the same scope even if
-- the config is later edited or deleted.
ALTER TABLE database_backup_configs
    ADD COLUMN target_database TEXT NOT NULL DEFAULT '';

ALTER TABLE database_backups
    ADD COLUMN target_database TEXT NOT NULL DEFAULT '';
