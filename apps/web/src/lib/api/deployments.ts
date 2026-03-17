import type { Deployment } from "@/lib/types";
import { api } from "./client";

export function listDeployments(projectId: string, serviceId: string) {
  return api.get<Deployment[]>(
    `/projects/${projectId}/services/${serviceId}/deployments`,
  );
}

export function getDeployment(
  projectId: string,
  serviceId: string,
  deploymentId: string,
) {
  return api.get<Deployment>(
    `/projects/${projectId}/services/${serviceId}/deployments/${deploymentId}`,
  );
}
