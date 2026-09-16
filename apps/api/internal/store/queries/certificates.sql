-- name: CreateCertificate :one
INSERT INTO certificates (name, cert_pem_encrypted, key_pem_encrypted, issuer, subjects, not_before, not_after, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetCertificate :one
SELECT * FROM certificates WHERE id = $1;

-- name: DeleteCertificate :exec
DELETE FROM certificates WHERE id = $1;

-- name: ListCertificates :many
-- Metadata plus the number of domains pointing at each certificate. The count
-- drives the UI's "in use" column and explains the 409 on a blocked delete; the
-- encrypted PEM columns come along but callers must never surface the key.
SELECT c.*, (
    SELECT COUNT(*) FROM domains d WHERE d.certificate_id = c.id
) AS domain_count
FROM certificates c
ORDER BY c.name;

-- name: ListCertificatesForHostname :many
-- Certificates that can serve one hostname, for the non-admin picker on the
-- add-domain form. A member may attach a certificate (the domain routes are
-- project-scoped) but may not LIST them, so without this they can only attach
-- one whose uuid an admin passed to them out of band.
--
-- ⚠️ Deliberately NOT SELECT *, and deliberately not c.subjects. A certificate
-- carries every SAN it was issued for, so returning the array would hand a
-- member the hostnames of every OTHER application it also covers — the exact
-- leak that keeps ListCertificates admin-only. The caller learns only that
-- SOMETHING covers the hostname they already supplied, plus what to call it and
-- when it expires. domain_count is omitted for the same reason: how widely a
-- certificate is used across the install is not a member's business.
--
-- Matching is an array overlap against candidates the caller's own hostname
-- generates (see certificateHostnameCandidates): the exact name, plus its
-- one-label wildcard parent. RFC 6125 wildcards match exactly one label and
-- only the leftmost, so "*.example.com" covers "app.example.com" but neither
-- "a.b.example.com" nor "example.com" — generating the candidate rather than
-- pattern-matching in SQL keeps that rule in one tested place.
SELECT id, name, issuer, not_after
FROM certificates
WHERE subjects && @candidates::text[]
ORDER BY name;

-- name: ListDomainsByCertificate :many
-- Names the domains still referencing a certificate, so a delete blocked by the
-- FK can tell the user exactly what is in the way.
SELECT hostname FROM domains WHERE certificate_id = $1 ORDER BY hostname;

-- name: ListCustomCertDomains :many
-- Every *hostname* serving an uploaded certificate, with the encrypted PEM pair.
-- The reconciler renders this into Caddy's load_pem set on each pass: a restarted
-- Caddy drops loaded certificates exactly as it drops routes.
--
-- DISTINCT ON hostname: a host split across paths has one domains row per path,
-- and Caddy selects a certificate by SNI, which knows nothing about paths. Left
-- as one row per domain, the same PEM pair would be decrypted and pushed into
-- load_pem once per path — needless crypto on every pass, and duplicate
-- certificates for one name in Caddy's store.
--
-- The oldest row wins, deterministically, so a host whose rows somehow disagree
-- about which certificate to serve does not flip between them from pass to pass.
SELECT DISTINCT ON (d.hostname) d.hostname, c.id AS certificate_id, c.cert_pem_encrypted, c.key_pem_encrypted
FROM domains d
JOIN certificates c ON c.id = d.certificate_id
WHERE d.ssl_mode = 'custom'
ORDER BY d.hostname, d.created_at ASC;
