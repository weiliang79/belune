-- Records whether a control-plane backup archive was encrypted with age at
-- creation time. Not derivable after the fact: remote_key only ends in .age
-- for UPLOADED runs (local-only runs have remote_key NULL), and the
-- encryption key can be set/unset over time, so a per-run flag is the only
-- accurate record of whether a SPECIFIC archive needs the private key to
-- restore. Both producers (worker and scripts/backup.sh) already know this
-- at encrypt time.
ALTER TABLE backup_runs
    ADD COLUMN encrypted BOOLEAN NOT NULL DEFAULT false;
