-- Optional base key prefix for a backup destination. The effective object key is
-- <destination.prefix>/<config.prefix>/<file>, so a destination can pin a base
-- path (e.g. a per-tenant folder) while each backup config adds its own subpath.
ALTER TABLE backup_destinations
    ADD COLUMN prefix TEXT NOT NULL DEFAULT '';
