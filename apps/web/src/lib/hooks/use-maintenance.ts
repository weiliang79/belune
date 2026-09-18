import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { queryKeys } from "./query-keys";
import * as maintenanceApi from "@/lib/api/maintenance";
import type {
  CleanupAction,
  PlatformService,
  RestartableService,
} from "@/lib/api/maintenance";

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

export function useServerIP() {
  return useQuery({
    queryKey: queryKeys.maintenanceServerIP,
    queryFn: maintenanceApi.getServerIP,
    staleTime: 60_000,
    refetchOnWindowFocus: false,
  });
}

export function useRestartService() {
  return useMutation({
    mutationFn: (service: RestartableService) =>
      maintenanceApi.restartService(service),
  });
}

export function useTriggerSelfUpdate() {
  return useMutation({
    mutationFn: ({ password, code }: { password: string; code?: string }) =>
      maintenanceApi.triggerSelfUpdate(password, code),
  });
}

export function usePlatformLogs(service: PlatformService | null) {
  return useQuery({
    queryKey: [...queryKeys.maintenancePlatformLogs, service],
    queryFn: () => maintenanceApi.getPlatformLogs(service as PlatformService),
    enabled: service !== null,
    refetchOnWindowFocus: false,
    staleTime: 0,
  });
}

export function useClearPendingQueue() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: maintenanceApi.clearPendingQueue,
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

/** Polls the last update attempt while one is in flight.
 *
 *  `enabled` is driven by the caller rather than always-on: this is admin-only
 *  and session-gated, so polling it on every Server-page render would 403 for a
 *  member and add a request nobody asked for. The 5s interval stops once the
 *  server reports anything other than "running" — a finished update replaces
 *  this container anyway, at which point the query refetches on reconnect. */
export function useSelfUpdateStatus(enabled: boolean) {
  return useQuery({
    queryKey: queryKeys.maintenanceUpdateStatus,
    queryFn: maintenanceApi.getSelfUpdateStatus,
    enabled,
    refetchInterval: (query) =>
      query.state.data?.state === "running" ? 5_000 : false,
  });
}

/** Queues an immediate manifest check, then refreshes settings so the card
 *  picks up the new cached values once the worker has written them. */
export function useTriggerUpdateCheck() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: maintenanceApi.triggerUpdateCheck,
    onSuccess: () => {
      // The worker writes the cache asynchronously; give it a moment before
      // re-reading, or the card refetches the PREVIOUS check's values and
      // looks like nothing happened.
      setTimeout(() => {
        void qc.invalidateQueries({ queryKey: queryKeys.settings });
      }, 1500);
    },
  });
}
