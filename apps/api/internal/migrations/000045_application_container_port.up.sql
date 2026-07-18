-- The internal port an image application's container listens on. Previously the
-- port lived only on the application's domain, so a domain-less image app (e.g. a
-- template deployed without a hostname) had no way to declare its port and the
-- deploy path defaulted the health check to 8080. Templates populate this from the
-- manifest; git-built apps leave it NULL and continue to resolve via the domain.
ALTER TABLE applications ADD COLUMN container_port INTEGER;
