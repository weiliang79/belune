import type { VolumeBackupConfig, VolumeBackup } from "@/lib/types";
import { api } from "./client";

export interface SaveVolumeBackupConfig {
  destination_id: string;
  prefix?: string;
  quiesce: boolean;
  enabled?: boolean;
}

function base(projectId: string, applicationId: string, volumeId: string) {
  return `/projects/${projectId}/applications/${applicationId}/volumes/${volumeId}`;
}

export function listVolumeBackupConfigs(
  projectId: string,
  applicationId: string,
  volumeId: string,
) {
  return api.get<VolumeBackupConfig[]>(
    `${base(projectId, applicationId, volumeId)}/backup-configs`,
  );
}

export function createVolumeBackupConfig(
  projectId: string,
  applicationId: string,
  volumeId: string,
  data: SaveVolumeBackupConfig,
) {
  return api.post<VolumeBackupConfig>(
    `${base(projectId, applicationId, volumeId)}/backup-configs`,
    data,
  );
}

export function deleteVolumeBackupConfig(
  projectId: string,
  applicationId: string,
  volumeId: string,
  configId: string,
) {
  return api.delete<{ status: string }>(
    `${base(projectId, applicationId, volumeId)}/backup-configs/${configId}`,
  );
}

export function runVolumeBackupConfig(
  projectId: string,
  applicationId: string,
  volumeId: string,
  configId: string,
) {
  return api.post<{ status: string }>(
    `${base(projectId, applicationId, volumeId)}/backup-configs/${configId}/run`,
    {},
  );
}

export function listVolumeBackups(
  projectId: string,
  applicationId: string,
  volumeId: string,
) {
  return api.get<VolumeBackup[]>(
    `${base(projectId, applicationId, volumeId)}/backups`,
  );
}

export function restoreVolumeBackup(
  projectId: string,
  applicationId: string,
  volumeId: string,
  backupId: string,
) {
  return api.post<{ status: string }>(
    `${base(projectId, applicationId, volumeId)}/backups/${backupId}/restore`,
    {},
  );
}
