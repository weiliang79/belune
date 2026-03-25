DELETE FROM metric_snapshots WHERE granularity = '1s';
ALTER TABLE metric_snapshots DROP CONSTRAINT metric_snapshots_granularity_check;
ALTER TABLE metric_snapshots ADD CONSTRAINT metric_snapshots_granularity_check
    CHECK (granularity IN ('1m', '5m', '1h'));
