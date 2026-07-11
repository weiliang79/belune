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

-- name: ListDomainsByCertificate :many
-- Names the domains still referencing a certificate, so a delete blocked by the
-- FK can tell the user exactly what is in the way.
SELECT hostname FROM domains WHERE certificate_id = $1 ORDER BY hostname;

-- name: ListCustomCertDomains :many
-- Every domain serving an uploaded certificate, with the encrypted PEM pair.
-- The reconciler renders this into Caddy's load_pem set on each pass: a restarted
-- Caddy drops loaded certificates exactly as it drops routes.
SELECT d.hostname, c.id AS certificate_id, c.cert_pem_encrypted, c.key_pem_encrypted
FROM domains d
JOIN certificates c ON c.id = d.certificate_id
WHERE d.ssl_mode = 'custom'
ORDER BY d.hostname;
