import type { Domain } from "@/lib/types";
import { api } from "./client";

export function listDomains(projectId: string, serviceId: string) {
  return api.get<Domain[]>(
    `/projects/${projectId}/services/${serviceId}/domains`,
  );
}

export function addDomain(
  projectId: string,
  serviceId: string,
  data: { hostname: string; ssl_enabled: boolean },
) {
  return api.post<Domain>(
    `/projects/${projectId}/services/${serviceId}/domains`,
    data,
  );
}

export function removeDomain(
  projectId: string,
  serviceId: string,
  domainId: string,
) {
  return api.delete<void>(
    `/projects/${projectId}/services/${serviceId}/domains/${domainId}`,
  );
}
