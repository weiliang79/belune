DROP INDEX IF EXISTS idx_databases_slug_per_project;
ALTER TABLE databases DROP COLUMN IF EXISTS slug;
