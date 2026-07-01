import type { BackupRun, BackupStatus } from "@/lib/types";
import { api } from "./client";

export function listBackupRuns() {
  return api.get<BackupRun[]>("/backups");
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
