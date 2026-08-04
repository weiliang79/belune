import type { BackupRemoteConfig, BackupRun, BackupStatus } from "@/lib/types";
import { api } from "./client";

export interface ListBackupRunsParams {
  limit?: number;
  offset?: number;
}

export function listBackupRuns(params?: ListBackupRunsParams) {
  const query = new URLSearchParams();
  if (params?.limit) query.set("limit", String(params.limit));
  if (params?.offset) query.set("offset", String(params.offset));
  const qs = query.toString();
  return api.get<BackupRun[]>(`/backups${qs ? `?${qs}` : ""}`);
}

export function getBackupStatus() {
  return api.get<BackupStatus>("/backups/status");
}

export function triggerBackupRun() {
  return api.post<{ status: string }>("/backups/run");
}

export function testBackupRemote() {
  return api.post<{ status: string }>("/backups/test");
}

export interface UpdateBackupRemoteData {
  enabled: boolean;
  endpoint: string;
  region: string;
  bucket: string;
  prefix: string;
  use_ssl: boolean;
  /** Blank preserves the currently stored secret. */
  access_key?: string;
  secret_key?: string;
}

export function updateBackupRemote(data: UpdateBackupRemoteData) {
  return api.put<{ status: string; remote: BackupRemoteConfig }>(
    "/backups/remote",
    data,
  );
}
