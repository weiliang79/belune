import type { EnvVar } from "@/lib/types";
import { api } from "./client";

export function getEnvVars(projectId: string, applicationId: string) {
  return api.get<EnvVar[]>(`/projects/${projectId}/applications/${applicationId}/env`);
}

export function upsertEnvVars(
  projectId: string,
  applicationId: string,
  vars: { key: string; value: string; is_secret: boolean }[],
) {
  return api.put<EnvVar[]>(`/projects/${projectId}/applications/${applicationId}/env`, {
    vars,
  });
}
