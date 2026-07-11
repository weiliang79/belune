-- Centralised certificate store: upload a PEM pair once, reference it from any
-- number of domains. Replaces the per-domain cert_path/key_path, which pointed
-- at files *inside the Caddy container* and so required the operator to
-- bind-mount PEMs by hand. Certificates now live here and are pushed to Caddy
-- over the admin API (load_pem), never touching its filesystem.
CREATE TABLE certificates (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name               VARCHAR(255) NOT NULL UNIQUE,
    -- Envelope-encrypted with the keyring, as backup destination credentials are.
    cert_pem_encrypted BYTEA NOT NULL,
    key_pem_encrypted  BYTEA NOT NULL,
    -- Metadata parsed out of the leaf at upload time so the UI can show issuer
    -- and expiry without ever decrypting the private key.
    issuer             TEXT,
    subjects           TEXT[] NOT NULL DEFAULT '{}',
    not_before         TIMESTAMPTZ,
    not_after          TIMESTAMPTZ,
    created_by         UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Expiry sweeps (notifications) scan by not_after.
CREATE INDEX idx_certificates_not_after ON certificates (not_after);

-- RESTRICT, not CASCADE: deleting a certificate that a live domain is serving
-- would silently break TLS for it. The handler turns this into a 409 naming the
-- domains still referencing it.
ALTER TABLE domains
    ADD COLUMN certificate_id UUID REFERENCES certificates(id) ON DELETE RESTRICT;

CREATE INDEX idx_domains_certificate_id ON domains (certificate_id);

-- Nothing functional ever read these: SetupTLS passed them to Caddy's load_files,
-- which resolves paths inside the Caddy container.
ALTER TABLE domains
    DROP COLUMN cert_path,
    DROP COLUMN key_path;
