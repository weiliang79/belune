-- Add slug column to databases table
ALTER TABLE databases ADD COLUMN slug VARCHAR(255) NOT NULL DEFAULT '';

-- Backfill slugs from name (lowercase, replace non-alphanumeric with hyphens)
UPDATE databases SET slug = LOWER(REGEXP_REPLACE(TRIM(name), '[^a-zA-Z0-9]+', '-', 'g'));

-- Remove default after backfill
ALTER TABLE databases ALTER COLUMN slug DROP DEFAULT;

-- Add unique index per project
CREATE UNIQUE INDEX idx_databases_slug_per_project ON databases(project_id, slug);
