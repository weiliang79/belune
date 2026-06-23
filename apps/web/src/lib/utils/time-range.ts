export type TimeRange = "24h" | "7d" | "30d" | "all";

const RANGE_MS: Record<Exclude<TimeRange, "all">, number> = {
  "24h": 24 * 60 * 60 * 1000,
  "7d": 7 * 24 * 60 * 60 * 1000,
  "30d": 30 * 24 * 60 * 60 * 1000,
};

/**
 * Resolve a range key to RFC3339 `from`/`to` bounds (empty for "all"). Compute
 * this once when the range changes (e.g. inside a filters useMemo) so the
 * derived bounds stay stable for a given selection.
 */
export function timeRangeToDates(range: TimeRange): {
  from?: string;
  to?: string;
} {
  if (range === "all") return {};
  const now = Date.now();
  return {
    from: new Date(now - RANGE_MS[range]).toISOString(),
    to: new Date(now).toISOString(),
  };
}
