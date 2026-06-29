-- Provider visibility: who (which users) may connect through a configured
-- provider. `created_by` records the admin who registered it; `is_public`
-- controls whether the provider is offered to every user in Connections
-- (true) or only to its owner admin (false).
--
-- Existing rows default to is_public = true with a NULL owner, preserving the
-- prior behavior where every configured provider was visible to all users.
ALTER TABLE git_provider_configs
    ADD COLUMN created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN is_public BOOLEAN NOT NULL DEFAULT true;
