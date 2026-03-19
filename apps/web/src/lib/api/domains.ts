import type { Domain } from "@/lib/types";
import { api } from "./client";

export function listDomains(projectId: string, applicationId: string) {
  return api.get<Domain[]>(
    `/projects/${projectId}/applications/${applicationId}/domains`,
  );
}

export function addDomain(
  projectId: string,
  applicationId: string,
  data: { hostname: string; ssl_enabled: boolean },
) {
  return api.post<Domain>(
    `/projects/${projectId}/applications/${applicationId}/domains`,
    data,
  );
}

export function removeDomain(
  projectId: string,
  applicationId: string,
  domainId: string,
) {
  return api.delete<void>(
    `/projects/${projectId}/applications/${applicationId}/domains/${domainId}`,
  );
}
