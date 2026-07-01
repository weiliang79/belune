import type { BackupDestination } from "@/lib/types";
import { api } from "./client";

export interface SaveBackupDestination {
  name: string;
  provider: string;
  endpoint?: string;
  region?: string;
  bucket: string;
  prefix?: string;
  use_ssl?: boolean;
  access_key?: string;
  secret_key?: string;
}

export function listBackupDestinations(projectId: string) {
  return api.get<BackupDestination[]>(
    `/projects/${projectId}/backup-destinations`,
  );
}

export function createBackupDestination(
  projectId: string,
  data: SaveBackupDestination,
) {
  return api.post<BackupDestination>(
    `/projects/${projectId}/backup-destinations`,
    data,
  );
}

export function updateBackupDestination(
  projectId: string,
  destId: string,
  data: SaveBackupDestination,
) {
  return api.put<BackupDestination>(
    `/projects/${projectId}/backup-destinations/${destId}`,
    data,
  );
}

export function deleteBackupDestination(projectId: string, destId: string) {
  return api.delete<{ status: string }>(
    `/projects/${projectId}/backup-destinations/${destId}`,
  );
}

export function testBackupDestination(projectId: string, destId: string) {
  return api.post<{ ok: boolean; error?: string }>(
    `/projects/${projectId}/backup-destinations/${destId}/test`,
    {},
  );
}

// testBackupDestinationParams tests ad-hoc form values before saving. Include
// `id` when editing so a blank secret falls back to the stored credentials.
export function testBackupDestinationParams(
  projectId: string,
  data: SaveBackupDestination & { id?: string },
) {
  return api.post<{ ok: boolean; error?: string }>(
    `/projects/${projectId}/backup-destinations/test`,
    data,
  );
}
