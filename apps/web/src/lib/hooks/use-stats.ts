import { useQuery } from "@tanstack/react-query";
import { queryKeys } from "./query-keys";
import { getStats } from "@/lib/api/stats";

/**
 * Operator-health stats for the dashboard strips. Polled on a slow interval —
 * these are at-a-glance health numbers, not a live feed.
 */
export function useStats() {
  return useQuery({
    queryKey: queryKeys.stats,
    queryFn: getStats,
    refetchInterval: 30_000,
  });
}
