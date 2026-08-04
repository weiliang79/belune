import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  listBackupRuns,
  getBackupStatus,
  triggerBackupRun,
  testBackupRemote,
  updateBackupRemote,
} from "@/lib/api/backups";
import { queryKeys } from "./query-keys";

export function useBackupRuns(params: { limit: number; offset: number }) {
  return useQuery({
    queryKey: queryKeys.backups.runs(params.limit, params.offset),
    queryFn: () => listBackupRuns(params),
    // Only the first page can show a run that just started, so only poll
    // there — polling a back page while a backup runs would be pointless.
    refetchInterval: (query) =>
      params.offset === 0 && query.state.data?.[0]?.status === "running"
        ? 5000
        : false,
  });
}

export function useBackupStatus() {
  return useQuery({
    queryKey: queryKeys.backups.status,
    queryFn: getBackupStatus,
  });
}

export function useTriggerBackup() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: triggerBackupRun,
    onSuccess: () => {
      const refresh = () => {
        qc.invalidateQueries({ queryKey: ["backups", "runs"] });
        qc.invalidateQueries({ queryKey: queryKeys.backups.status });
      };
      // The task is async: the 202 returns before the worker inserts the
      // "running" run row. A single invalidate would refetch too early and
      // miss it, so re-poll a few times until the row (and its result) appear.
      refresh();
      [1000, 2500, 5000].forEach((ms) => setTimeout(refresh, ms));
    },
  });
}

export function useTestBackupRemote() {
  return useMutation({ mutationFn: testBackupRemote });
}

export function useUpdateBackupRemote() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: updateBackupRemote,
    onSuccess: () =>
      qc.invalidateQueries({ queryKey: queryKeys.backups.status }),
  });
}
