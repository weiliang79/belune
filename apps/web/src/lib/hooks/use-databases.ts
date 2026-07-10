import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { queryKeys } from "./query-keys";
import * as databasesApi from "@/lib/api/databases";

export function useDatabases(projectId: string) {
  return useQuery({
    queryKey: queryKeys.databases.all(projectId),
    queryFn: () => databasesApi.listDatabases(projectId),
    // Fast while a database is provisioning; slow floor otherwise so changes
    // from other sessions still surface without hammering the API.
    refetchInterval: (query) =>
      query.state.data?.some(
        (d) =>
          d.status === "creating" ||
          d.status === "upgrading" ||
          d.status === "backing_up",
      )
        ? 3000
        : 30000,
  });
}

export function useDatabase(projectId: string, databaseId: string) {
  return useQuery({
    queryKey: queryKeys.databases.detail(projectId, databaseId),
    queryFn: () => databasesApi.getDatabase(projectId, databaseId),
    refetchInterval: (query) => {
      const status = query.state.data?.status;
      return status === "creating" ||
        status === "upgrading" ||
        status === "backing_up"
        ? 3000
        : false;
    },
  });
}

export function useCreateDatabase(projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: Parameters<typeof databasesApi.createDatabase>[1]) =>
      databasesApi.createDatabase(projectId, data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.databases.all(projectId) });
      qc.invalidateQueries({ queryKey: queryKeys.projects.detail(projectId) });
    },
  });
}

export function useDeleteDatabase(projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (databaseId: string) =>
      databasesApi.deleteDatabase(projectId, databaseId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.databases.all(projectId) });
      qc.invalidateQueries({ queryKey: queryKeys.projects.detail(projectId) });
    },
  });
}

// Lifecycle hooks refresh both the database detail and the project's database
// list so the row status updates without waiting for the next poll.
function useDatabaseLifecycle(
  projectId: string,
  databaseId: string,
  mutationFn: () => Promise<unknown>,
) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn,
    // Await both refetches so the mutation stays `isPending` until the row's
    // status reflects the change — otherwise the action buttons briefly flash
    // the wrong state (e.g. Stop/Restart) between the spinner and Start.
    onSuccess: () =>
      Promise.all([
        qc.invalidateQueries({
          queryKey: queryKeys.databases.detail(projectId, databaseId),
        }),
        qc.invalidateQueries({ queryKey: queryKeys.databases.all(projectId) }),
      ]),
    onError: (error: Error) => toast.error(error.message),
  });
}

export function useStopDatabase(projectId: string, databaseId: string) {
  return useDatabaseLifecycle(projectId, databaseId, () =>
    databasesApi.stopDatabase(projectId, databaseId),
  );
}

export function useStartDatabase(projectId: string, databaseId: string) {
  return useDatabaseLifecycle(projectId, databaseId, () =>
    databasesApi.startDatabase(projectId, databaseId),
  );
}

export function useRestartDatabase(projectId: string, databaseId: string) {
  return useDatabaseLifecycle(projectId, databaseId, () =>
    databasesApi.restartDatabase(projectId, databaseId),
  );
}

export function useUpdateDatabase(projectId: string, databaseId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: { cpu_limit: number; memory_limit: number }) =>
      databasesApi.updateDatabase(projectId, databaseId, data),
    onSuccess: () => {
      qc.invalidateQueries({
        queryKey: queryKeys.databases.detail(projectId, databaseId),
      });
    },
  });
}

export function useSetDatabaseExternalAccess(
  projectId: string,
  databaseId: string,
) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (enabled: boolean) =>
      databasesApi.setDatabaseExternalAccess(projectId, databaseId, enabled),
    onSuccess: () => {
      qc.invalidateQueries({
        queryKey: queryKeys.databases.detail(projectId, databaseId),
      });
    },
  });
}

export function useDatabaseBackups(projectId: string, databaseId: string) {
  return useQuery({
    queryKey: queryKeys.databases.backups(projectId, databaseId),
    queryFn: () => databasesApi.listDatabaseBackups(projectId, databaseId),
    // Poll faster while a backup is running so the row resolves promptly.
    refetchInterval: (query) =>
      query.state.data?.some((b) => b.status === "running") ? 3000 : false,
  });
}

export function useBackupDatabase(projectId: string, databaseId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => databasesApi.backupDatabase(projectId, databaseId),
    onSuccess: () => {
      const key = queryKeys.databases.backups(projectId, databaseId);
      // The worker inserts the "running" run row shortly after the task is
      // enqueued; re-poll briefly so it shows up without a manual refresh.
      qc.invalidateQueries({ queryKey: key });
      [1000, 2500, 5000].forEach((ms) =>
        setTimeout(() => qc.invalidateQueries({ queryKey: key }), ms),
      );
    },
  });
}

export function useDeleteDatabaseBackup(projectId: string, databaseId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (backupId: string) =>
      databasesApi.deleteDatabaseBackup(projectId, databaseId, backupId),
    onSuccess: () => {
      qc.invalidateQueries({
        queryKey: queryKeys.databases.backups(projectId, databaseId),
      });
    },
  });
}

export function useDatabaseRestores(projectId: string, databaseId: string) {
  return useQuery({
    queryKey: queryKeys.databases.restores(projectId, databaseId),
    queryFn: () => databasesApi.listDatabaseRestores(projectId, databaseId),
    refetchInterval: (query) =>
      query.state.data?.some((r) => r.status === "running") ? 3000 : false,
  });
}

export function useRestoreDatabase(projectId: string, databaseId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (backupId: string) =>
      databasesApi.restoreDatabase(projectId, databaseId, backupId),
    onSuccess: () => {
      // The worker inserts the "running" restore row shortly after enqueue;
      // re-poll briefly so it shows up without a manual refresh.
      const key = queryKeys.databases.restores(projectId, databaseId);
      qc.invalidateQueries({ queryKey: key });
      [1000, 2500, 5000].forEach((ms) =>
        setTimeout(() => qc.invalidateQueries({ queryKey: key }), ms),
      );
    },
  });
}

export function useUpgradeDatabase(projectId: string, databaseId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (targetVersion: string) =>
      databasesApi.upgradeDatabase(projectId, databaseId, targetVersion),
    onSuccess: () => {
      qc.invalidateQueries({
        queryKey: queryKeys.databases.detail(projectId, databaseId),
      });
    },
  });
}
