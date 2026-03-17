import type { EnvVar } from "@/lib/types";
import { api } from "./client";

export function getEnvVars(projectId: string, serviceId: string) {
  return api.get<EnvVar[]>(`/projects/${projectId}/services/${serviceId}/env`);
}

export function upsertEnvVars(
  projectId: string,
  serviceId: string,
  vars: { key: string; value: string; is_secret: boolean }[],
) {
  return api.put<EnvVar[]>(`/projects/${projectId}/services/${serviceId}/env`, {
    vars,
  });
}
