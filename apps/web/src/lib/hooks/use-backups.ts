import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { listBackupRuns, getBackupStatus, triggerBackupRun } from "@/lib/api/backups";
import { queryKeys } from "./query-keys";

export function useBackupRuns() {
  return useQuery({
    queryKey: queryKeys.backups.runs,
    queryFn: listBackupRuns,
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
      qc.invalidateQueries({ queryKey: queryKeys.backups.runs });
      qc.invalidateQueries({ queryKey: queryKeys.backups.status });
    },
  });
}
