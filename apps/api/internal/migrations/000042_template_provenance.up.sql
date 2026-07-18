-- Provenance for objects created from an app template (or, later, a compose
-- import). Stored generously, shown sparingly (design memory): enables
-- template-update notices, support, and analytics; unrecoverable if skipped.
--
--   source_kind: how the object was created — 'template', later 'compose-import'.
--                NULL for the manual/UI path (the overwhelming majority).
--   source_ref:  which source, e.g. 'umami@1' (template id @ manifest schema).
--
-- Both nullable; no backfill needed (NULL = created manually).
ALTER TABLE applications ADD COLUMN source_kind VARCHAR(32),
                         ADD COLUMN source_ref  VARCHAR(255);

ALTER TABLE databases ADD COLUMN source_kind VARCHAR(32),
                      ADD COLUMN source_ref  VARCHAR(255);
