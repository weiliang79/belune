-- Add swap columns to host_metrics so the Server page can show swap alongside
-- RAM. Swap is never summed with RAM (a sum reads healthiest exactly when the box
-- is thrashing) — these are stored and charted as a separate series. Defaults keep
-- existing rows valid.
ALTER TABLE host_metrics
    ADD COLUMN swap_used  BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN swap_total BIGINT NOT NULL DEFAULT 0;
