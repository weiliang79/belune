-- The internal path: a prefix prepended to the request before it reaches the
-- app, for an app that insists on serving everything under a base path of its
-- own (Grafana under /grafana, a service whose routes all live under /app).
--
-- It composes with strip_path, in that order: strip the public prefix, then
-- prepend the internal one. Together they are a genuine remap —
--   public shop.com/public/api/users
--   strip /public        -> /api/users
--   prepend /app/v2      -> /app/v2/api/users   (what the container receives)
--
-- Empty means "prepend nothing", which is what every existing domain does.
ALTER TABLE domains ADD COLUMN internal_path VARCHAR(255) NOT NULL DEFAULT '';

-- Empty, or rooted. A bare "app" would prepend to "/users" as "app/users",
-- which is not a path at all — and Caddy would forward it without complaint.
ALTER TABLE domains ADD CONSTRAINT domains_internal_path_rooted
    CHECK (internal_path = '' OR internal_path LIKE '/%');
