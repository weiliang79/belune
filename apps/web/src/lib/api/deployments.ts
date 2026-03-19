import type { Deployment } from "@/lib/types";
import { api } from "./client";

export function listDeployments(projectId: string, applicationId: string) {
  return api.get<Deployment[]>(
    `/projects/${projectId}/applications/${applicationId}/deployments`,
  );
}

export function getDeployment(
  projectId: string,
  applicationId: string,
  deploymentId: string,
) {
  return api.get<Deployment>(
    `/projects/${projectId}/applications/${applicationId}/deployments/${deploymentId}`,
  );
}
