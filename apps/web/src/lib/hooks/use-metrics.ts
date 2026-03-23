import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { queryKeys } from "./query-keys";
import * as metricsApi from "@/lib/api/metrics";

export function useMetrics() {
  return useQuery({
    queryKey: queryKeys.metrics,
    queryFn: metricsApi.getMetrics,
    refetchInterval: 30_000,
  });
}

export function useTriggerCleanup() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (retainCount?: number) => metricsApi.triggerCleanup(retainCount),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.metrics }),
  });
}

export function useHostHistoricalMetrics(range: string) {
  return useQuery({
    queryKey: queryKeys.hostMetrics(range),
    queryFn: () => metricsApi.getHostHistoricalMetrics(range),
    refetchInterval: 60_000,
  });
}

export function useApplicationMetrics(
  projectId: string,
  applicationId: string,
  range: string,
) {
  return useQuery({
    queryKey: queryKeys.appMetrics(projectId, applicationId, range),
    queryFn: () => metricsApi.getApplicationMetrics(projectId, applicationId, range),
    refetchInterval: 60_000,
  });
}
