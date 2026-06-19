-- "Other" database type: run an arbitrary container image as a managed database.
-- Known engines (postgres/mysql/redis/mongo) keep deriving image/port/data-dir
-- from per-engine defaults; for type='other' these come from the user and are
-- stored here. backup_mode selects how an "other" database is backed up.
ALTER TABLE databases DROP CONSTRAINT databases_type_check;
ALTER TABLE databases ADD CONSTRAINT databases_type_check
    CHECK (type IN ('postgres', 'mysql', 'redis', 'mongo', 'other'));

ALTER TABLE databases
    ADD COLUMN image          TEXT,
    ADD COLUMN container_port  INTEGER,
    ADD COLUMN data_dir        TEXT,
    ADD COLUMN backup_mode     TEXT NOT NULL DEFAULT 'none'
        CHECK (backup_mode IN ('none', 'volume_snapshot', 'command')),
    ADD COLUMN backup_command  TEXT,
    ADD COLUMN restore_command TEXT;
