-- 'local' is a certificate issued by Caddy's own internal CA, when Caddy is
-- *configured* to do that (local_certs) rather than falling back to it.
--
-- The distinction matters and is the whole reason this is a separate status. In
-- production, a certificate from "Caddy Local Authority" means automatic HTTPS
-- has not obtained a real one — either it is still working or it has failed —
-- and calling that active would tell the operator HTTPS is fine when no browser
-- would trust it. So it stays pending there, deliberately.
--
-- In development Caddy runs with local_certs and the internal certificate is the
-- final state, not a waypoint: pending would never resolve. Reporting 'local'
-- says what is true without weakening the production rule, and it is decided by
-- reading Caddy's own configured issuer rather than by guessing from the issuer
-- name — which is exactly what a fallback would look like.
ALTER TABLE domains DROP CONSTRAINT IF EXISTS domains_tls_status_check;

ALTER TABLE domains ADD CONSTRAINT domains_tls_status_check
    CHECK (tls_status IN (
        'unknown', 'disabled', 'pending', 'active',
        'expiring', 'expired', 'failed', 'local'
    ));
