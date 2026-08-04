-- Adds a "local" backup destination provider: a destination row with no
-- bucket/endpoint/credentials that tells the worker to keep the staged
-- archive on-host (under belunedata) instead of uploading it. Lets DB/volume
-- backup schedules go local-only, matching what control-plane backups could
-- already do and what backup-destinations.mdx has always documented ("or
-- none, if you're keeping backups local only") but the schema never allowed.
ALTER TABLE backup_destinations DROP CONSTRAINT backup_destinations_provider_check;
ALTER TABLE backup_destinations ADD CONSTRAINT backup_destinations_provider_check
    CHECK (provider IN ('s3', 'r2', 'b2', 'wasabi', 'minio', 'other', 'local'));
