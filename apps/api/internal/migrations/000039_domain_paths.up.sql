-- Path-based routing: serve one hostname from several applications, split by
-- URL prefix (shop.com/api -> the API, shop.com/ -> the frontend).
--
-- `path` is the public prefix the domain answers on. `strip_path` decides
-- whether the prefix is removed before the request reaches the app — an app
-- mounted at /api usually expects to see /users, not /api/users.
ALTER TABLE domains ADD COLUMN path VARCHAR(255) NOT NULL DEFAULT '/';
ALTER TABLE domains ADD COLUMN strip_path BOOLEAN NOT NULL DEFAULT false;

-- A path is a prefix, so it must be rooted. Without this an empty string or a
-- bare "api" would produce a Caddy matcher that silently never matches.
ALTER TABLE domains ADD CONSTRAINT domains_path_rooted CHECK (path LIKE '/%');

-- The heart of the change. hostname was globally UNIQUE (twice over: a table
-- constraint and a unique index), which makes two rows for one host impossible
-- and so makes path routing impossible. Uniqueness moves to the pair.
--
-- Existing rows all take path='/', which is exactly what they do today: a
-- host-only matcher answers every path. So this is a no-op for every domain
-- that currently exists.
ALTER TABLE domains DROP CONSTRAINT IF EXISTS domains_hostname_key;
DROP INDEX IF EXISTS idx_domains_hostname;

ALTER TABLE domains ADD CONSTRAINT domains_hostname_path_key UNIQUE (hostname, path);

-- hostname alone is still looked up constantly — the TLS sweep, the Caddy log
-- parser, the access-log tailer all key on it — so it keeps an index, just no
-- longer a unique one.
CREATE INDEX idx_domains_hostname ON domains (hostname);
