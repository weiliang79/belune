-- Fix bloated application slugs created by a bug where the finalSlug was
-- stored as "{projectSlug}-{baseSlug}-{id[:8]}" instead of just "{baseSlug}".
-- This strips the project-slug prefix and the 8-char hex ID suffix.
UPDATE applications a
SET slug = regexp_replace(
    substr(a.slug, length(p.slug) + 2),
    '-[0-9a-f]{8}$', ''
)
FROM projects p
WHERE a.project_id = p.id
  AND a.slug ~ ('^' || p.slug || '-.+-[0-9a-f]{8}$');
