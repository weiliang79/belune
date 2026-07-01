import type { ProjectBackupActivity } from "@/lib/types";
import { api } from "./client";

export function listProjectBackups(projectId: string) {
  return api.get<ProjectBackupActivity[]>(`/projects/${projectId}/backups`);
}
