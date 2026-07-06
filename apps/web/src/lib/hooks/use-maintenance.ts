import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { queryKeys } from "./query-keys";
import * as maintenanceApi from "@/lib/api/maintenance";
import type { CleanupAction } from "@/lib/api/maintenance";

export function useReconcilerStatus() {
  return useQuery({
    queryKey: queryKeys.proxyReconciler,
    queryFn: maintenanceApi.getReconcilerStatus,
    refetchInterval: 30_000,
  });
}

export function useReconcileProxy() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: maintenanceApi.reconcileProxy,
    onSuccess: () =>
      qc.invalidateQueries({ queryKey: queryKeys.proxyReconciler }),
  });
}

export function useQueueStatus() {
  return useQuery({
    queryKey: queryKeys.maintenanceQueue,
    queryFn: maintenanceApi.getQueueStatus,
    refetchInterval: 15_000,
  });
}

export function useClearQueue() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: maintenanceApi.clearQueue,
    onSuccess: () =>
      qc.invalidateQueries({ queryKey: queryKeys.maintenanceQueue }),
  });
}

export function useRunCleanup() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (actions?: CleanupAction[]) =>
      maintenanceApi.runCleanup(actions),
    onSuccess: () => {
      // Cleanup runs async; refresh disk usage + metrics so the reclaimable
      // figures update once the worker finishes.
      qc.invalidateQueries({ queryKey: queryKeys.docker.overview });
      qc.invalidateQueries({ queryKey: queryKeys.metrics });
    },
  });
}
