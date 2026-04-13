-- Restore full slug format: "{projectSlug}-{baseSlug}-{id[:8]}"
-- Migration 000002 incorrectly stripped the prefix and suffix; this restores them.
-- Only runs on slugs that don't already end in "-{8hex}" (i.e. the stripped ones).
UPDATE applications a
SET slug = p.slug || '-' || a.slug || '-' || left(a.id::text, 8)
FROM projects p
WHERE a.project_id = p.id
  AND a.slug !~ '-[0-9a-f]{8}$';
