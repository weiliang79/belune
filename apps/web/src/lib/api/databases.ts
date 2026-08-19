import type { Database, DatabaseBackup, DatabaseRestore } from "@/lib/types";
import { api } from "./client";

export function listDatabases(projectId: string) {
  return api.get<Database[]>(`/projects/${projectId}/databases`);
}

export function getDatabase(projectId: string, databaseId: string) {
  return api.get<Database>(`/projects/${projectId}/databases/${databaseId}`);
}

export function getDatabaseVolume(projectId: string, databaseId: string) {
  return api.get<{ name: string; size_bytes: number | null }>(
    `/projects/${projectId}/databases/${databaseId}/volume`,
  );
}

export function createDatabase(
  projectId: string,
  data: {
    name: string;
    slug?: string;
    type: string;
    version?: string;
    credentials?: {
      user?: string;
      password?: string;
      database_name?: string;
      root_password?: string;
    };
    // "other" type only:
    image?: string;
    container_port?: number;
    data_dir?: string;
    env?: Record<string, string>;
    backup_mode?: "volume_snapshot" | "command";
    backup_command?: string;
    restore_command?: string;
  },
) {
  return api.post<Database>(`/projects/${projectId}/databases`, data);
}

export function updateDatabase(
  projectId: string,
  databaseId: string,
  data: { cpu_limit: number; memory_limit: number },
) {
  return api.put<Database>(
    `/projects/${projectId}/databases/${databaseId}`,
    data,
  );
}

export function setDatabaseExternalAccess(
  projectId: string,
  databaseId: string,
  enabled: boolean,
) {
  return api.post<{ status: string }>(
    `/projects/${projectId}/databases/${databaseId}/external-access`,
    { enabled },
  );
}

export function listDatabaseBackups(projectId: string, databaseId: string) {
  return api.get<DatabaseBackup[]>(
    `/projects/${projectId}/databases/${databaseId}/backups`,
  );
}

export function backupDatabase(projectId: string, databaseId: string) {
  return api.post<{ status: string }>(
    `/projects/${projectId}/databases/${databaseId}/backups`,
    {},
  );
}

export function listDatabaseRestores(projectId: string, databaseId: string) {
  return api.get<DatabaseRestore[]>(
    `/projects/${projectId}/databases/${databaseId}/restores`,
  );
}

export function restoreDatabase(
  projectId: string,
  databaseId: string,
  backupId: string,
) {
  return api.post<{ status: string }>(
    `/projects/${projectId}/databases/${databaseId}/restore`,
    { backup_id: backupId },
  );
}

export function deleteDatabaseBackup(
  projectId: string,
  databaseId: string,
  backupId: string,
) {
  return api.delete<{ status: string }>(
    `/projects/${projectId}/databases/${databaseId}/backups/${backupId}`,
  );
}

export function upgradeDatabase(
  projectId: string,
  databaseId: string,
  targetVersion: string,
) {
  return api.post<{ status: string }>(
    `/projects/${projectId}/databases/${databaseId}/upgrade`,
    { target_version: targetVersion },
  );
}

export function deleteDatabase(projectId: string, databaseId: string) {
  return api.delete<void>(`/projects/${projectId}/databases/${databaseId}`);
}

export function stopDatabase(projectId: string, databaseId: string) {
  return api.post<Database>(
    `/projects/${projectId}/databases/${databaseId}/stop`,
  );
}

export function startDatabase(projectId: string, databaseId: string) {
  return api.post<Database>(
    `/projects/${projectId}/databases/${databaseId}/start`,
  );
}

export function restartDatabase(projectId: string, databaseId: string) {
  return api.post<Database>(
    `/projects/${projectId}/databases/${databaseId}/restart`,
  );
}

// Recreate the container from the stored record (image, env, volume, network).
// Unlike restart, this recovers a container that has drifted or been deleted.
export function reloadDatabase(projectId: string, databaseId: string) {
  return api.post<{ status: string }>(
    `/projects/${projectId}/databases/${databaseId}/reload`,
  );
}

/** What deleting a database destroys beyond the database itself. */
export interface DatabaseDeletionImpact {
  backup_count: number;
  backup_destinations: string[];
}

export function getDatabaseDeletionImpact(
  projectId: string,
  databaseId: string,
) {
  return api.get<DatabaseDeletionImpact>(
    `/projects/${projectId}/databases/${databaseId}/deletion-impact`,
  );
}
