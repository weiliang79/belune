import type { EnvVar, EnvVarInput } from "@/lib/types";
import { api } from "./client";

export function getProjectEnvVars(projectId: string) {
  return api.get<EnvVar[]>(`/projects/${projectId}/env`);
}

export function upsertProjectEnvVars(projectId: string, vars: EnvVarInput[]) {
  return api.put<EnvVar[]>(`/projects/${projectId}/env`, { vars });
}

export function revealProjectEnvVar(projectId: string, envVarId: string) {
  return api.get<{ value: string }>(
    `/projects/${projectId}/env/${envVarId}/reveal`,
  );
}
