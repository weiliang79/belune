import type { EnvVar } from "@/lib/types";
import { api } from "./client";

export function getProjectEnvVars(projectId: string) {
  return api.get<EnvVar[]>(`/projects/${projectId}/env`);
}

export function upsertProjectEnvVars(
  projectId: string,
  vars: { key: string; value: string; is_secret: boolean }[],
) {
  return api.put<EnvVar[]>(`/projects/${projectId}/env`, { vars });
}
