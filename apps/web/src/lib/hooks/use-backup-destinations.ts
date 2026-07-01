import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { queryKeys } from "./query-keys";
import * as destApi from "@/lib/api/backup-destinations";
import type { SaveBackupDestination } from "@/lib/api/backup-destinations";

export function useBackupDestinations(projectId: string) {
  return useQuery({
    queryKey: queryKeys.backupDestinations(projectId),
    queryFn: () => destApi.listBackupDestinations(projectId),
  });
}

export function useCreateBackupDestination(projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: SaveBackupDestination) =>
      destApi.createBackupDestination(projectId, data),
    onSuccess: () =>
      qc.invalidateQueries({
        queryKey: queryKeys.backupDestinations(projectId),
      }),
  });
}

export function useUpdateBackupDestination(projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      destId,
      data,
    }: {
      destId: string;
      data: SaveBackupDestination;
    }) => destApi.updateBackupDestination(projectId, destId, data),
    onSuccess: () =>
      qc.invalidateQueries({
        queryKey: queryKeys.backupDestinations(projectId),
      }),
  });
}

export function useDeleteBackupDestination(projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (destId: string) =>
      destApi.deleteBackupDestination(projectId, destId),
    onSuccess: () =>
      qc.invalidateQueries({
        queryKey: queryKeys.backupDestinations(projectId),
      }),
  });
}

export function useTestBackupDestination(projectId: string) {
  return useMutation({
    mutationFn: (destId: string) =>
      destApi.testBackupDestination(projectId, destId),
  });
}

export function useTestBackupDestinationParams(projectId: string) {
  return useMutation({
    mutationFn: (data: SaveBackupDestination & { id?: string }) =>
      destApi.testBackupDestinationParams(projectId, data),
  });
}
