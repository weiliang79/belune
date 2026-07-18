-- Per-application container runtime profile (v0.0.32-alpha).
--
-- Belune hardens app containers by default (v0.0.9): read-only rootfs + all
-- capabilities dropped. That is the right default for untrusted code (the
-- operator's own git builds and manually-added images), but it breaks most
-- stock third-party images, which run a root entrypoint that chowns a data dir
-- and drops to a non-root user (needs CHOWN/SETUID/SETGID) and write to paths
-- outside /tmp and /run. Curated template images are smoke-tested and trusted,
-- so they default to a "standard" (Docker-default) runtime instead.
--
--   readonly_rootfs: true  -> read-only rootfs + tmpfs /tmp,/run (hardened)
--                    false -> writable rootfs, no extra tmpfs (standard)
--   container_caps:  'minimal'  -> CapDrop=ALL (hardened)
--                    'standard' -> Docker default capability set
--
-- no-new-privileges stays on in both profiles (it doesn't break the
-- privilege-drop startup idiom). Defaults below preserve the existing hardened
-- behaviour for every current app and for manually-created ones; the template
-- instantiation flow sets these to the standard profile on the apps it creates.
ALTER TABLE applications
    ADD COLUMN readonly_rootfs BOOLEAN     NOT NULL DEFAULT TRUE,
    ADD COLUMN container_caps  VARCHAR(16) NOT NULL DEFAULT 'minimal';
