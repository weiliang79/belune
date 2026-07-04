import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { queryKeys } from "./query-keys";
import * as api from "@/lib/api/volume-backups";

export function useVolumeBackupConfigs(
  projectId: string,
  applicationId: string,
  volumeId: string,
  enabled = true,
) {
  return useQuery({
    queryKey: queryKeys.volumeBackupConfigs(projectId, applicationId, volumeId),
    queryFn: () =>
      api.listVolumeBackupConfigs(projectId, applicationId, volumeId),
    enabled,
  });
}

export function useVolumeBackups(
  projectId: string,
  applicationId: string,
  volumeId: string,
  enabled = true,
) {
  return useQuery({
    queryKey: queryKeys.volumeBackups(projectId, applicationId, volumeId),
    queryFn: () => api.listVolumeBackups(projectId, applicationId, volumeId),
    enabled,
  });
}

export function useVolumeRestores(
  projectId: string,
  applicationId: string,
  volumeId: string,
  enabled = true,
) {
  return useQuery({
    queryKey: queryKeys.volumeRestores(projectId, applicationId, volumeId),
    queryFn: () => api.listVolumeRestores(projectId, applicationId, volumeId),
    enabled,
  });
}

export function useCreateVolumeBackupConfig(
  projectId: string,
  applicationId: string,
  volumeId: string,
) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: api.SaveVolumeBackupConfig) =>
      api.createVolumeBackupConfig(projectId, applicationId, volumeId, data),
    onSuccess: () =>
      qc.invalidateQueries({
        queryKey: queryKeys.volumeBackupConfigs(
          projectId,
          applicationId,
          volumeId,
        ),
      }),
  });
}

export function useUpdateVolumeBackupConfig(
  projectId: string,
  applicationId: string,
  volumeId: string,
  configId: string,
) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: api.SaveVolumeBackupConfig) =>
      api.updateVolumeBackupConfig(
        projectId,
        applicationId,
        volumeId,
        configId,
        data,
      ),
    onSuccess: () =>
      qc.invalidateQueries({
        queryKey: queryKeys.volumeBackupConfigs(
          projectId,
          applicationId,
          volumeId,
        ),
      }),
  });
}

export function useDeleteVolumeBackupConfig(
  projectId: string,
  applicationId: string,
  volumeId: string,
) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (configId: string) =>
      api.deleteVolumeBackupConfig(projectId, applicationId, volumeId, configId),
    onSuccess: () =>
      qc.invalidateQueries({
        queryKey: queryKeys.volumeBackupConfigs(
          projectId,
          applicationId,
          volumeId,
        ),
      }),
  });
}

export function useRunVolumeBackup(
  projectId: string,
  applicationId: string,
  volumeId: string,
) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (configId: string) =>
      api.runVolumeBackupConfig(projectId, applicationId, volumeId, configId),
    onSuccess: () => {
      // The run row is inserted async; re-poll shortly so it appears.
      const key = queryKeys.volumeBackups(projectId, applicationId, volumeId);
      [1000, 3000, 6000].forEach((ms) =>
        setTimeout(() => qc.invalidateQueries({ queryKey: key }), ms),
      );
    },
  });
}

export function useRestoreVolumeBackup(
  projectId: string,
  applicationId: string,
  volumeId: string,
) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (backupId: string) =>
      api.restoreVolumeBackup(projectId, applicationId, volumeId, backupId),
    onSuccess: () => {
      // The restore run row is inserted async; re-poll shortly so it appears.
      const key = queryKeys.volumeRestores(projectId, applicationId, volumeId);
      [1000, 3000, 6000].forEach((ms) =>
        setTimeout(() => qc.invalidateQueries({ queryKey: key }), ms),
      );
    },
  });
}
