-- Per-application file/config mounts.
--
-- Unlike volumes (persistent named Docker volumes), a file mount injects a
-- single config file whose CONTENT the user provides in the UI (config.yaml,
-- nginx.conf, an init script, a cert). On deploy the worker writes the
-- decrypted content to a managed host file under <paasDir>/filemounts/<app-id>/
-- and bind-mounts it read-only into the container at mount_path. The DB row is
-- the source of truth; the host file is regenerated on every deploy.
--
-- content is ALWAYS keyring-encrypted at rest (mirrors env_vars.value_encrypted).
-- is_secret masks the content in API responses (the file is still written on
-- deploy). mount_path is an absolute in-container FILE path, unique per app.
CREATE TABLE application_file_mounts (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id    UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    mount_path        VARCHAR(500) NOT NULL,
    content_encrypted BYTEA NOT NULL,
    is_secret         BOOLEAN NOT NULL DEFAULT false,
    file_mode         VARCHAR(4) NOT NULL DEFAULT '0644',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(application_id, mount_path)
);

CREATE INDEX idx_application_file_mounts_app ON application_file_mounts(application_id);
