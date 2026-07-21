import { useQuery } from "@tanstack/react-query";
import { queryKeys } from "./query-keys";
import { getStats } from "@/lib/api/stats";

/**
 * Operator-health stats for the dashboard strips. Polled on a slow interval —
 * these are at-a-glance health numbers, not a live feed.
 *
 * Refetched on every mount, overriding the global 60s staleTime. These counts
 * aggregate every application and database, so they are invalidated by actions
 * taken on pages that do *not* render this strip — stop an app in Project
 * Details, come back to Projects, and the cached "4/4 running" would otherwise
 * stand until the poll happened to fire. Refetching on arrival keeps that
 * correct without every service mutation having to remember to invalidate
 * ["stats"]. The strip already polls while mounted, so this adds no meaningful
 * traffic.
 */
export function useStats() {
  return useQuery({
    queryKey: queryKeys.stats,
    queryFn: getStats,
    refetchInterval: 30_000,
    staleTime: 0,
    refetchOnMount: "always",
  });
}
